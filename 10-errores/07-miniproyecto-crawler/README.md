# Mini-proyecto 3: crawler web concurrente

## Enunciado

Un crawler que recorre un sitio a partir de una URL semilla y que:

- limita la **concurrencia** (peticiones simultáneas),
- limita la **tasa** (peticiones por segundo),
- aplica un **timeout por petición** y **reintentos** con backoff,
- **no repite** URLs, no sale del host y respeta máximos de **páginas** y **profundidad**,
- se **cancela** limpiamente (`Ctrl+C` o plazo global) sin fugas,
- devuelve un **informe** (incluso parcial) con resultado por página.

Reúne casi todo lo visto: goroutines, channels como semáforo, `context`, `sync`, `atomic`, `errgroup`, `rate`, pruebas con `httptest`.

```bash
go run .                                   # sitio sintético local (60 páginas con casos problemáticos)
go run . -conc 1                           # secuencial
go run . -rate 20                          # 20 peticiones/s
go run . -url https://ejemplo.com -max 20  # sitio real (sé respetuoso: poco volumen y con tasa baja)
go test -race -count=3 -timeout 120s .
```

| Flag | Descripción | Defecto |
|--|--|--|
| `-url` | semilla (vacío = sitio sintético) | — |
| `-conc` | peticiones simultáneas | 8 |
| `-rate` | peticiones/s (0 = sin límite) | 0 |
| `-timeout` | plazo de cada petición | 150 ms |
| `-max` | máximo de páginas | 100 |
| `-prof` | profundidad máxima | 10 |
| `-reintentos` | reintentos por página | 1 |

## Diseño

```
         ┌──────────────────────────────────────────────┐
semilla ─► programar(url) ─► g.Go(visitar)               │  una goroutine por URL (acotada por MaxPaginas)
         │                      │                        │
         │        sem (chan, cap=MaxConcurrencia)  ◄── límite de CONCURRENCIA
         │                      │
         │        rate.Limiter.Wait (cada intento)  ◄── límite de TASA
         │                      │
         │        peticion: WithTimeout + Do + LimitReader(1 MB)
         │                      │ 5xx / error de red ⇒ reintento con backoff
         │                      ▼
         │        guardar Pagina ──► ExtraerEnlaces ──► programar(enlace…)
         └──────────────────────────────────────────────┘
                      errgroup.WithContext  (cancelación y espera estructurada)
```

Decisiones y por qué:

1. **Una goroutine por URL con semáforo**, no un pool fijo con cola. Un pool clásico plantea el problema de la **detección de fin** (¿cuándo cerrar la cola si los workers siguen añadiendo enlaces?). Con `errgroup`, una tarea puede llamar a `g.Go` para las páginas descubiertas mientras siga viva, y `Wait` termina solo cuando se agota todo el árbol. El coste es una goroutine por página, acotada por `MaxPaginas`.
2. **Deduplicación y tope de páginas bajo un mutex** en `programar`: se decide *antes* de lanzar, así dos páginas que enlazan a la misma URL no la visitan dos veces.
3. **El semáforo se libera antes de encolar los enlaces** para no retener una plaza mientras no se descarga nada.
4. **El limitador de tasa se aplica por intento**, incluido el reintento (un reintento también es una petición al servidor).
5. **Timeout por petición** con `context.WithTimeout` derivado del contexto global; la cancelación global sigue funcionando.
6. **Un error de página no es un error del crawl.** Cada página tiene su `Err`; las tareas solo devuelven error si el contexto se cancela. Así una página rota no aborta el recorrido.
7. **Reintentos solo ante fallos transitorios**: error de red/timeout y 5xx. Un 404 no se reintenta.
8. **Backoff exponencial** con espera cancelable (`select` con `ctx.Done()`).
9. **Informe parcial**: si se cancela, `Crawl` devuelve lo recorrido junto con el error del contexto.
10. **Límite de lectura de 1 MB** por página: protección frente a respuestas gigantes.
11. **Medición de la concurrencia real** (contador atómico), tanto en el cliente como en el servidor de pruebas.

### Sitio sintético (`sitio.go`)

Un `httptest.Server` con 60 páginas enlazadas en grafo, con un enlace externo, uno duplicado, un fragmento y un `mailto:`. Con `Especiales`:

| Ruta | Comportamiento |
|--|--|
| `/p/5` | 500 la primera vez, luego correcta (se recupera al reintentar) |
| `/p/7` | tarda 400 ms (agota el timeout) |
| `/p/13` | 404 (no se reintenta) |
| `/p/21` | siempre 500 (falla tras reintentar) |

## Resultado de referencia

```
60 páginas en 385ms (57 correctas, 3 con error) | máx. simultáneas: 8

páginas con error:
  .../p/13   estado=404 intentos=1 err=HTTP 404
  .../p/21   estado=500 intentos=2 err=HTTP 500
  .../p/7    estado=0   intentos=2 err=Get ".../p/7": context deadline exceeded

3 más lentas:
  .../p/7    321ms
```

Efecto de los límites sobre las mismas 60 páginas:

| Configuración | Tiempo | Máx. simultáneas |
|--|--|--|
| `-conc 8` (defecto) | 385 ms | 8 |
| `-conc 1` | 1.67 s | 1 |
| `-rate 20` | 3.12 s (63 peticiones ÷ 20/s) | 2 |

## Tests (`crawler_test.go`, 10 ejecuciones seguidas con `-race`)

| Test | Verifica |
|--|--|
| `ExtraerEnlaces` | resolución relativa, fragmentos, duplicados, `mailto:`/`javascript:`/`ftp:` ignorados |
| `RecorreTodoUnaVez` | 60 páginas, **cada ruta pedida exactamente 1 vez** en el servidor, sin salir del host |
| `LimiteDeConcurrencia` | con límite 3, servidor y cliente ven ≤ 3, y > 1 (se aprovecha) |
| `LimiteDeTasa` | 21 peticiones a 100/s tardan ≥ 180 ms |
| `MaxPaginas` / `Profundidad` | exactamente 10 páginas / 3 con profundidad 1 |
| `PaginasProblematicas` | `/p/5` se recupera (2 intentos), `/p/13` 404 sin reintentos, `/p/21` 500 tras reintentar, `/p/7` agota el plazo, y el resto sigue |
| `CancelacionRapidaYSinFugas` | con plazo de 80 ms y latencia de 300 ms: devuelve `DeadlineExceeded` rápido, con informe parcial y sin goroutines fugadas |
| `SemillaInvalida` | rechaza `""`, texto libre, `ftp://` y rutas relativas |

## Limitaciones conocidas (extensiones)

1. **HTML con regexp.** Útil para el ejercicio; para sitios reales usa `golang.org/x/net/html`.
2. **Sin `robots.txt`** ni `User-Agent` identificable: imprescindibles para rastrear sitios reales con educación.
3. **Una goroutine por URL.** Con millones de páginas, cambia a un pool con cola persistente y detección de fin con contador atómico de "trabajo pendiente".
4. **Un único limitador global.** Para varios hosts, uno por host (`map[string]*rate.Limiter`).
5. **Sin persistencia ni reanudación**: guardar el estado de `vistas` y la cola.
6. **Sin respeto a `Retry-After` / 429**: leer la cabecera y pausar.
7. **Redirecciones y canonicalización**: dos URLs distintas pueden ser la misma página (`/a` y `/a/`).
8. **Contenido no HTML** (PDF, imágenes): comprobar `Content-Type` antes de leer el cuerpo.
9. **Métricas**: histograma de latencias y contadores por estado.
