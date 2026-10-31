# Proyecto final: `jobq`, una cola de trabajos distribuida

Un **job scheduler** (opción 2 del sílabo) construido con lo visto en los 15 módulos: un **coordinador** que gestiona una cola con prioridades, reintentos, límite de tasa y contrapresión, y **N workers** (contenedores) que arriendan y ejecutan trabajos. El paralelismo real se consigue **repartiendo workers en contenedores con Docker Compose**.

```
                      ┌────────────────────────── coordinador ──────────────────────────┐
  loadgen / clientes  │  API HTTP/JSON                                                  │
  POST /jobs ────────►│   Enviar ─► idempotencia ─► capacidad (503) ─► cuota tenant (429)│
                      │                  │                                              │
                      │                  ▼          ┌─────────────┐   Run(ctx): reloj   │
                      │        4 colas FIFO por     │ línea de    │◄── (min-heap de     │
                      │        prioridad + aging    │ tiempo      │    reintentos y     │
                      │                  │          │ (eventos)   │    arriendos)       │
   long polling       │        Arrendar ◄┘          └─────────────┘                     │
  ┌─ POST /lease ────►│   estado: pendiente → en_curso → completado                      │
  │                   │                 ▲   │   retardado (backoff) ─┐ → fallido (DLQ)  │
  │  ack / nack /     │                 └───┴────────────────────────┘                  │
  │  heartbeat ──────►│   /metrics   /stats   /debug/pprof   /healthz                    │
  │                   └─────────────────────────────────────────────────────────────────┘
  │
  ├── worker-1 (1 CPU) ─ ranuras ─ [hash][hash]      Cada worker: bucle de arriendo +
  ├── worker-2 (1 CPU) ─ ranuras ─ [hash][hash]      N ranuras + latidos + timeouts +
  └── worker-N …                                     recover + apagado con gracia
```

## Por qué este proyecto

Es el que más conceptos del curso obliga a usar *a la vez* y de forma realista:

| Módulo | Dónde aparece en `jobq` |
|--|--|
| 1-2 goroutines, `WaitGroup.Go`, GOMAXPROCS | ranuras del worker; `GOMAXPROCS` consciente de cgroups en los contenedores |
| 3 channels | difusión «ha llegado trabajo» (canal que se cierra y se reemplaza) compatible con `ctx` |
| 4 select / timers | long polling con `time.Timer`; reloj del planificador con un único temporizador |
| 5 context | `Run(ctx)` en todos los componentes; plazos por trabajo; `WithCancelCause` para el arriendo perdido; `WithoutCancel` para informar tras cancelar |
| 6 sync | `sync.Mutex` en el planificador; secciones críticas cortas; sin `Cond` (no es cancelable) |
| 7 atomic | contadores e histogramas de latencia sin cerrojo (`atomic.Int64`) |
| 8 patrones | worker pool con semáforo, productor-consumidor, rate limiting (token bucket), pub-sub de señal, apagado ordenado |
| 9 iteradores | `slices.Sort` para resumir latencias en `loadgen`; el diseño de la API evita exponer iteradores o canales porque el sondeo largo ya da la semántica de flujo |
| 10 errores | errores de dominio con `errors.Is/As`, traducción ↔ HTTP, errores permanentes frente a reintentables |
| 11 testing | `testing/synctest` (reloj virtual: backoff exacto), `goleak`, `-race`, `httptest` |
| 12 profiling | `net/http/pprof`, `/metrics` en formato Prometheus, benchmarks |
| 13 rendimiento | batching de arriendos, contadores atómicos, medición de speedup con contenedores |
| 14 internals | `GOMAXPROCS` y cgroups, long polling = goroutines aparcadas (sin hilos), timers |
| 15 diseño avanzado | prioridades + **envejecimiento** (inanición), **backoff con jitter** (livelock/estampida), **contrapresión**, **idempotencia**, APIs `Run(ctx)`, concurrencia estructurada (`errgroup`), apagado ordenado HTTP |

## Cómo ejecutarlo

### Con Docker Compose (recomendado)

```bash
cd proyecto-final
docker compose up -d --build                    # coordinador + 2 workers (1 CPU cada uno)
docker compose up -d --scale worker=4           # escalar a 4 workers
docker compose run --rm loadgen                 # 1 000 trabajos de CPU con prioridades mezcladas
curl localhost:8080/stats | jq .                # estado interno
curl localhost:8080/metrics                     # formato Prometheus
go tool pprof -top -seconds 10 http://localhost:8080/debug/pprof/profile
docker compose down
```

Variables útiles (exportarlas **para todos** los comandos `docker compose`, porque `run` vuelve a evaluar la configuración y recrearía el coordinador con los valores por defecto):

```bash
export CAPACIDAD=100 TASA_TENANT=50
docker compose up -d --force-recreate coordinador
```

Experimento de escalado (speedup al añadir contenedores):

```bash
./scripts/escalado.sh 300 600000 "1 2 4 8"
```

### Sin Docker

```bash
go run ./cmd/coordinador -addr :8080 &
go run ./cmd/worker -url http://localhost:8080 -slots 4 &
go run ./cmd/loadgen -url http://localhost:8080 -n 400 -tipo dormir -payload '{"ms":20}' -mix
```

### Tests y benchmarks

```bash
make test        # go vet + go test -race ./...
make race        # -count=5
make bench       # benchmarks del planificador con -cpu 1,4,8
```

## API

| Método y ruta | Qué hace | Respuestas |
|--|--|--|
| `POST /jobs` | encola `{tipo, payload, prioridad 0-3, tenant, clave, max_intentos, timeout_ms}` | `201` creado · `200` repetido (misma `clave`) · `400` · **`429`** cuota del tenant (+`Retry-After`) · **`503`** cola llena (+`Retry-After`) |
| `GET /jobs/{id}[?esperar=10s]` | estado del trabajo; con `esperar`, sondeo largo hasta que termine | `200` · `404` |
| `DELETE /jobs/{id}` | cancela si aún no empezó | `204` · `409` |
| `POST /lease` | `{worker, max, espera_ms}` → sondeo largo; devuelve hasta `max` trabajos arrendados | `200` (lista, quizá vacía) |
| `POST /jobs/{id}/ack` · `/nack` · `/heartbeat` | terminar bien · fallar (`reintentable`) · renovar el arriendo | `204` · **`410`** arriendo perdido |
| `GET /stats` · `/metrics` · `/healthz` · `/debug/pprof/*` | observabilidad | |

Tipos de trabajo de demostración: `eco`, `dormir {ms}` (E/S), `hash {rondas}` (CPU), `fallo {prob, permanente}` (reintentos), `panico`.

## Decisiones de diseño

### 1. Modelo de arriendo (*lease*), no de «empujar»
Los workers **piden** trabajo (`/lease`, sondeo largo). Así el coordinador no necesita conocer a los workers, el balanceo es natural (quien está libre, pide) y los workers lentos no acumulan cola. El trabajo se **arrienda** por un TTL: si el worker muere, el arriendo **caduca** y el trabajo vuelve a la cola (entrega **al menos una vez**, *at-least-once*). Un worker vivo renueva con **latidos**. Un `Ack` de un arrendatario que ya perdió el arriendo recibe `410` y su resultado se descarta. Consecuencia: **los handlers deben ser idempotentes**.

### 2. Un solo `sync.Mutex` y un reloj con una línea de tiempo
Todo el estado está bajo un `Mutex`; las secciones críticas son cortas (mapas y colas). Los eventos futuros (reintentos, caducidad de arriendos, purga) van en un **min-heap** servido por **una sola goroutine** (`Run`) con **un único `time.Timer`**: no hay un temporizador ni una goroutine por trabajo. Los eventos obsoletos no se borran: se invalidan por una **generación** (*lazy deletion*) y se descartan al vencer.

### 3. Sondeo largo sin `sync.Cond`
Los workers ociosos esperan en un canal de difusión que se **cierra y se reemplaza** al llegar trabajo (solo si hay alguien esperando). A diferencia de `Cond`, es compatible con `select`, `ctx` y temporizador, y ese `select` es lo que hace baratas 1 000 esperas (goroutines aparcadas, sin hilos; módulo 14).

### 4. Prioridades con envejecimiento
Cuatro colas FIFO (una por nivel). Se atiende la cabeza de **mayor prioridad efectiva** = `nivel + espera/Envejecimiento`. Sin envejecimiento, un flujo continuo de trabajos urgentes deja sin servicio a los de baja prioridad (**inanición**, módulo 15); con él, la espera compensa la diferencia de nivel. Es O(niveles) por arriendo.

### 5. Reintentos con *backoff* exponencial y *jitter*
`espera = base · 2^(intento-1)`, con tope y **jitter** («equal jitter»: mitad fija + mitad aleatoria) para evitar la estampida de reintentos sincronizados. Los errores se clasifican en **reintentables** y **permanentes** (`worker.ErrPermanente`); agotados los intentos, el trabajo pasa a `fallido` (cola de muertos) conservando el último error.

### 6. Contrapresión y cuotas
- `Capacidad` acota los trabajos en espera: al llenarse, `503` + `Retry-After`. «Fallar pronto y barato» frente a acumular latencia (módulos 8 y 15).
- Un **token bucket por tenant** (`golang.org/x/time/rate`) responde `429` + `Retry-After` con el tiempo exacto, **sin consumir la ficha** del rechazo.
- Orden de comprobaciones: idempotencia → capacidad → cuota. Un reenvío idempotente no gasta cuota ni capacidad; un rechazo por cola llena no gasta ficha.

### 7. Idempotencia en el envío
La `clave` evita duplicados cuando un cliente reintenta tras un timeout. Se libera con la purga del trabajo.

### 8. Ciclo de vida estructurado y apagado ordenado
- Todos los componentes siguen **`Run(ctx)` bloqueante**; `main` los une con `errgroup` y `signal.NotifyContext`.
- **Coordinador:** `Shutdown` espera a las solicitudes en curso (incluidos los sondeos largos).
- **Worker:** al recibir `SIGTERM` deja de arrendar, espera la **gracia** a los trabajos en curso y, si se agota, los cancela y los **devuelve a la cola** (`Nack` reintentable «worker apagándose»). Los trabajos cuelgan de un contexto *separado* (`WithoutCancel`) precisamente para tener esa gracia.
- **Un pánico en un handler** se convierte en un error del trabajo; no mata al worker.
- **Pérdida del arriendo** (`410` en un latido): se cancela el trabajo con `WithCancelCause` para no gastar CPU en algo que otro worker repetirá.

### 9. Observabilidad
Contadores e histogramas **atómicos** (sin cerrojo), `/stats` (JSON), `/metrics` (texto Prometheus escrito a mano, sin dependencias) y `pprof` siempre disponible.

### 10. Contenedores
Una imagen con los tres binarios estáticos; cada servicio elige el suyo. Los workers tienen **`cpus: 1.0`**. Go ≥ 1.25 ajusta `GOMAXPROCS` al límite de CPU del cgroup (aquí **2**: el mínimo que fija el runtime), de modo que el paralelismo viene de **añadir contenedores**, no de añadir goroutines dentro de uno.

## Resultados medidos (Apple M1, Docker Desktop con 8 CPUs, Go 1.26.5)

### Escalado con contenedores (`scripts/escalado.sh 300 600000`: 300 trabajos de CPU, ~85 ms cada uno)

```
workers   total      jobs/s   speedup   eficiencia
1         26.1 s       11      1.00x      100 %
2         13.3 s       23      2.09x      105 %
4          6.6 s       46      4.18x      105 %
8          3.7 s       81      7.36x       92 %
```

### Tolerancia a fallos (matar un worker con `SIGKILL` a mitad de la carga)
600 trabajos `dormir 200 ms` con 4 workers; a los 5 s, `docker kill jobq-worker-2`:
`completados 600 · fallidos 0 · reintentos 2 · arriendos_caducados 2` (las 2 ranuras del worker muerto: sus trabajos caducaron a los 10 s y los terminaron otros workers).

### Apagado ordenado (`docker compose stop worker` con trabajos de 3 s en curso)
Los 4 workers registran `worker detenido` en 2 s y la carga termina con `completados 40 · reintentos 0`: ningún trabajo se perdió ni se repitió.

### Contrapresión (600 trabajos `dormir 50 ms`, 128 envíos simultáneos, 4 workers)

| Capacidad | rechazos 503 reintentados | throughput | latencia p50 extremo a extremo |
|--|--|--|--|
| 50 000 (sin límite efectivo) | 0 | 154 jobs/s | **2.0 s** |
| **100** | 2 351 | 154 jobs/s | **0.67 s** |

El mismo throughput con **3× menos latencia**: la cola acotada sólo traslada la espera al cliente (que reintenta con *jitter*), en vez de acumularla dentro del sistema.

### Prioridades con envejecimiento (400 trabajos `dormir 20 ms`, mezcla 10/20/40/30 %, 1 worker con 4 ranuras)

```
prioridad 3 (crítica):  32 trabajos | p50 103 ms | p95 186 ms | máx 187 ms
prioridad 2:            71 trabajos | p50 384 ms | p95 553 ms | máx 575 ms
prioridad 1:           196 trabajos | p50 1.09 s | p95 1.56 s | máx 1.63 s
prioridad 0 (baja):    101 trabajos | p50 1.89 s | p95 2.12 s | máx 2.15 s
```
Las prioridades altas se sirven antes, y la baja **termina** (máx. 2.15 s) gracias al envejecimiento.

### Rendimiento del sistema y del planificador
- **Planificador aislado** (sin red): `Enviar` + `Arrendar` + `Ack` ≈ **1.4-1.6 µs/op** (~700 000 ciclos/s) y **no escala** con `-cpu` (1 595 → 1 416 ns con 8): es un único `Mutex`, la opción conocida más simple; el diseño admite fragmentarlo (ver ejercicios).
- **Sistema completo** con trabajos `eco` (sobrecarga pura): **≈ 13 500 trabajos/s** de extremo a extremo con 4 workers y 128 conexiones.
- **Perfil de CPU del coordinador** a ese ritmo (`pprof`, 6 s): **38 % en llamadas al sistema** (E/S de red), ~6 % en el recolector de basura y la asignación; el planificador y su `Mutex` **no aparecen** entre los 16 primeros. El cuello de botella es HTTP, no el cerrojo, y por eso fragmentar el planificador **no** sería la primera optimización.

### Pruebas
`go test -race ./...`: 42 tests (synctest con reloj virtual para backoff, caducidad de arriendos, envejecimiento y long polling; `httptest` + workers reales para el extremo a extremo; estrés de 8 productores × 8 consumidores con «exactamente una vez»; `goleak` en todos los paquetes).

## Estructura

```
proyecto-final/
├── cmd/{coordinador,worker,loadgen}/main.go
├── internal/
│   ├── modelo/         tipos, estados y errores del dominio
│   ├── planificador/   el corazón: colas, línea de tiempo, arriendos, métricas
│   ├── api/            servidor HTTP, cliente tipado, apagado ordenado
│   └── worker/         bucle de arriendo, ranuras, latidos, handlers de demostración
├── Dockerfile · compose.yaml · Makefile
└── scripts/escalado.sh
```

## Limitaciones conocidas (y por dónde seguir)

- **Estado en memoria:** si el coordinador cae, se pierde la cola. *Siguiente paso:* un registro de operaciones (WAL) con *fsync* por lotes y recuperación al arrancar, o Redis/Postgres (`SELECT … FOR UPDATE SKIP LOCKED`) como almacén.
- **Un solo coordinador** (punto único de fallo y de escalado vertical). *Siguiente paso:* fragmentar por tenant/hash de la clave (módulo 15, mapa fragmentado) o replicar con Raft.
- **Entrega al menos una vez:** los handlers deben tolerar repeticiones.
- **La ficha del tenant** se reserva antes de insertar; todo ocurre bajo el mismo cerrojo, así que no hay ventana de carrera, pero el límite es por proceso.
- **Sin autenticación ni TLS:** pensado para una red interna de contenedores.

## Ejercicios propuestos

1. **Persistencia:** añade un WAL (`Enviar`/`Ack`/`Nack` como líneas JSON con *fsync* por lotes) y recupera el estado al arrancar. Prueba matando el coordinador con `docker kill`.
2. **Fragmentar el planificador** por `hash(tenant) % N` con N mutex y N colas, y mide si cambia el techo de ≈ 700 000 ciclos/s (¿y el de 13 500 trabajos/s del sistema completo?).
3. **Lotes en el ack:** `POST /ack` con varios identificadores; mide cuánto sube el throughput de `eco` (módulo 13: amortizar costes fijos).
4. **Dependencias entre trabajos** (DAG): un trabajo solo se encola cuando terminan sus predecesores.
5. **Trabajos periódicos** (cron) reutilizando la línea de tiempo.
6. **Límite de concurrencia por tipo** (p. ej. máx. 10 `hash` simultáneos en todo el sistema) con un semáforo distribuido basado en arriendos.
7. **Prometheus + Grafana** en el `compose.yaml` apuntando a `/metrics`.
8. **`GOMAXPROCS` y `-slots`:** compara `cpus: 1.0` con `-slots 1` y con `-slots 8` para trabajos de CPU y de E/S: ¿qué ranuras conviene en cada caso?
