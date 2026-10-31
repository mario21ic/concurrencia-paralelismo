# Ejercicio 4: traza de ejecución (`runtime/trace`)

## Teoría

Un perfil **muestrea** (cada ~10 ms); una **traza registra todos los eventos** del runtime con marcas de tiempo de nanosegundos: creación y cambios de estado de cada goroutine, bloqueos y desbloqueos, GC, llamadas al sistema, qué P ejecuta qué. Responde preguntas que ningún perfil puede:

- ¿Mis goroutines **aprovechan todos los núcleos** o hay huecos?
- ¿Cuánto esperan **listas para ejecutar** (saturación del planificador)?
- ¿Qué pasó **exactamente** durante esa petición lenta?
- ¿Quién desbloqueó a quién?

### Cómo obtenerla

```go
f, _ := os.Create("trace.out")
trace.Start(f); defer trace.Stop()
```

o `go test -trace=trace.out`, o `/debug/pprof/trace?seconds=5`.

### Cómo analizarla

| Herramienta | Qué da |
|--|--|
| `go tool trace trace.out` | **visor web**: línea de tiempo por P y por goroutine, histogramas, "goroutine analysis" |
| `go tool trace -pprof=sync\|sched\|net\|syscall trace.out` | perfiles tipo pprof derivados de la traza: dónde se bloquea (`sync`), latencia de planificación (`sched`), red y *syscalls* |
| `golang.org/x/exp/trace` | lector programático (el que usa `internal/traza` en este módulo para resumir sin navegador) |

### Anotaciones del usuario

Una traza cruda tiene miles de goroutines anónimas. Las anotaciones le dan significado de negocio:

| API | Concepto |
|--|--|
| `trace.NewTask(ctx, "pedido")` | **tarea**: una unidad lógica de trabajo que puede cruzar goroutines (p. ej. una petición) |
| `trace.WithRegion(ctx, "validar", f)` / `StartRegion` | **región**: un tramo con nombre dentro de **una** goroutine |
| `trace.Log(ctx, "cat", "msg")` / `Logf` | **mensaje** fechado asociado a la tarea |

El visor permite ver, por ejemplo, "la latencia de cada tarea `pedido` y cuánto fue ejecución, E/S o espera en cada región".

### Coste

Trazar tiene coste (más cuanto más eventos). En este ejercicio fue ~6 %; en cargas con muchísimas goroutines y cambios de estado puede ser mayor. Para tenerla **siempre activa** sin guardar todo, existe el *flight recorder* (ejercicio 5).

## Qué hace el programa

400 "pedidos" repartidos entre 8 workers. Cada pedido es una **tarea** con tres **regiones**:

| Región | Qué hace |
|--|--|
| `validar` | 0.3 ms de CPU |
| `esperar-bd` | `Sleep` de 2 ms (E/S simulada) |
| `guardar` | 0.4 ms **con un mutex global** (se serializa) |

Se ejecuta primero sin traza (referencia) y luego con ella, y se analiza la traza con `internal/traza`.

```bash
go run . -workers 8 -salida trace.out
go tool trace trace.out
go tool trace -pprof=sync trace.out > sync.pprof && go tool pprof -top sync.pprof
```

## Resultado de referencia

```
sin traza: 170ms | con traza: 181ms | archivo trace.out: 63 KB          <- coste ≈ 6 %

eventos: 10870 | duración de la traza: 181ms | goroutines observadas: 17 | máx. goroutines listas a la vez: 8

TIEMPO ACUMULADO DE GOROUTINES POR ESTADO
  Waiting   1.369s
  Running    295ms
  Runnable    56ms

ESPERAS (Waiting) POR MOTIVO
  sleep          838ms        <- la E/S simulada (400 × ~2.1 ms)
  sync           287ms        <- esperas por el mutex de "guardar"
  chan send      142ms        <- el productor esperando a que haya worker libre
  chan receive   102ms

REGIONES
  nombre          veces     total     media      máx.
  esperar-bd        400   838.9ms   2.097ms   4.514ms
  guardar           400   468.0ms   1.170ms   6.928ms     <- ¡media 1.17 ms para 0.4 ms de trabajo!
  validar           400   126.9ms     317µs   1.604ms

TAREAS
  pedido            400   1.436s    3.59ms    9.81ms
```

**Perfil `sync` derivado de la traza** (coincide con el análisis propio):

```
284ms 53.5%  sync.(*Mutex).Lock
142ms 26.7%  runtime.chansend1
100ms 18.8%  runtime.chanrecv1
```

## Conclusiones

- **`guardar` tarda 1.17 ms de media pero su trabajo real es 0.4 ms**: los ~0.77 ms restantes son espera por el lock. El resumen lo muestra sin abrir el visor, y `-pprof=sync` lo atribuye a `sync.(*Mutex).Lock` (284 ms ≈ los 287 ms de `sync` medidos).
- `validar` dura 317 µs frente a 300 µs de CPU: casi no hay espera de planificación (`Runnable` solo 56 ms en total).
- Una **tarea** `pedido` mide 3.6 ms de media; las regiones permiten repartir ese tiempo entre CPU (`validar`), E/S (`esperar-bd`) y contención (`guardar`).
- Con la traza se ve lo que el perfil de CPU **no** muestra: el ~80 % del tiempo acumulado de las goroutines (`Waiting`: 1.37 s de 1.72 s) no era CPU.

## Tests

El analizador `internal/traza` se prueba generando una traza real con tareas, regiones, logs y esperas conocidas (`TestAnalizar`) y comprobando que una traza inválida da error.

## Ejercicios propuestos

1. Abre `trace.out` en `go tool trace` y localiza visualmente las esperas de `guardar` en la línea de tiempo de los Ps.
2. Sustituye el mutex global de `guardar` por uno fragmentado y compara `guardar` (media y máx.).
3. Ejecuta con `-workers 1` y con `-workers 64`: ¿cómo cambia `máx. goroutines listas a la vez`?
4. Añade `trace.WithRegion` a tu propio servicio del módulo 8 y busca el cuello de botella.
