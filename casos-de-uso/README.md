# Casos de uso: concurrencia y paralelismo por **problema**, de básico a avanzado

Los módulos del curso (`01-fundamentos` … `15-diseno-avanzado`) están organizados por **feature del lenguaje**: `sync`, `context`, `channels`… Es la forma correcta de *aprender cada herramienta*, pero en un proyecto real el problema no llega con la etiqueta «necesito `sync.RWMutex`». Llega así: *«tengo que descargar 50 URLs»*, *«esto se rompe de vez en cuando»*, *«¿cuántas goroutines pongo?»*.

Esta carpeta **invierte el orden**: parte del **problema** y enseña **qué feature elegir, cuándo y por qué**, con código que se ejecuta, tests y un README por caso que remite al módulo donde profundizar.

## Cómo usarla

1. Elige el **concepto** que te interesa (o sigue la [ruta sugerida](#ruta-de-estudio-sugerida)).
2. Dentro, recorre los casos **1 → 2 → 3** (básico → intermedio → avanzado): cada uno añade un problema nuevo al anterior.
3. En cada caso: ejecuta `go run .`, lee la salida, lee el código (comentado con el *porqué*), mira los *Errores típicos* y salta a los módulos de *Profundiza*.
4. Haz los ejercicios de *Prueba tú*.

```bash
cd casos-de-uso/01-lanzar-y-esperar/1-basico-waitgroup
go run .            # qué hace
go test -race -v .  # comprobaciones (muchas con tiempo virtual: testing/synctest)
```

## Mapa de conceptos

| # | Concepto | Pregunta que responde | Tipo | 1 · básico | 2 · intermedio | 3 · avanzado |
|--|--|--|--|--|--|--|
| 1 | [Lanzar y esperar](01-lanzar-y-esperar) | ¿cómo hago varias cosas a la vez y sé cuándo acaban? | concurrencia | [WaitGroup](01-lanzar-y-esperar/1-basico-waitgroup) | [errgroup](01-lanzar-y-esperar/2-intermedio-errgroup) | [degradación elegante](01-lanzar-y-esperar/3-avanzado-degradacion) |
| 2 | [Estado compartido](02-estado-compartido) | ¿cómo evito que se pisen al compartir un dato? | concurrencia | [mutex / atomic](02-estado-compartido/1-basico-mutex) | [caché + singleflight](02-estado-compartido/2-intermedio-cache) | [contadores fragmentados](02-estado-compartido/3-avanzado-contadores) |
| 3 | [Comunicar datos](03-comunicar-datos) | ¿cómo paso datos y monto un flujo de etapas? | concurrencia | [productor-consumidor](03-comunicar-datos/1-basico-productor-consumidor) | [pipeline](03-comunicar-datos/2-intermedio-pipeline) | [fan-out ordenado](03-comunicar-datos/3-avanzado-fan-out-ordenado) |
| 4 | [Tiempo y cancelación](04-tiempo-y-cancelacion) | ¿cómo evito esperar para siempre, reintento y apago? | concurrencia | [timeout](04-tiempo-y-cancelacion/1-basico-timeout) | [reintentos](04-tiempo-y-cancelacion/2-intermedio-reintentos) | [apagado ordenado](04-tiempo-y-cancelacion/3-avanzado-apagado-ordenado) |
| 5 | [Limitar recursos](05-limitar-recursos) | ¿cómo acoto concurrencia, cola y tasa? | concurrencia | [semáforo](05-limitar-recursos/1-basico-semaforo) | [worker pool](05-limitar-recursos/2-intermedio-worker-pool) | [rate limit](05-limitar-recursos/3-avanzado-rate-limit) |
| 6 | [Paralelismo de CPU](06-paralelismo-cpu) | ¿cómo uso todos los núcleos (y cuándo no compensa)? | **paralelismo** | [suma paralela](06-paralelismo-cpu/1-basico-suma-paralela) | [map-reduce](06-paralelismo-cpu/2-intermedio-map-reduce) | [merge sort](06-paralelismo-cpu/3-avanzado-merge-sort) |
| 7 | [E/S frente a CPU](07-io-vs-cpu) | ¿cuántos workers? ¿cuándo salgo del proceso? | **paralelismo** | [cuántos workers](07-io-vs-cpu/1-basico-cuantos-workers) | — | [contenedores](07-io-vs-cpu/2-avanzado-contenedores) |

### Concurrencia ≠ paralelismo (en estos casos)

- **Concurrencia** = *estructurar* un programa en tareas independientes que **se solapan** (conceptos 1-5). Funciona con 1 núcleo: lo que se solapa son **esperas**.
- **Paralelismo** = *ejecutar a la vez* en varios núcleos para **terminar antes un cálculo** (conceptos 6-7). Solo ayuda si hay cálculo que repartir.
- Un programa puede tener una sin la otra: la suma paralela (6.1) es paralelismo; descargar 8 URLs a la vez (1.1) es concurrencia (con 1 solo núcleo tardaría casi lo mismo).

## ¿Qué herramienta uso? (guía rápida)

```
¿Qué necesito?
│
├─ Hacer varias cosas a la vez ........................... go + WaitGroup.Go ........ caso 1.1
│    └─ …y parar al primer error o limitar cuántas ........ errgroup ................. caso 1.2
│
├─ Varias goroutines tocan el MISMO dato
│    ├─ es UN número / puntero, operación simple ........... sync/atomic ............. caso 2.1
│    ├─ es una estructura / varios campos .................. sync.Mutex .............. caso 2.1
│    ├─ se lee mucho, se escribe poco ...................... sync.RWMutex (¡mide!) .... caso 2.2
│    ├─ muchas peticiones por la misma clave cara .......... singleflight ............ caso 2.2
│    └─ MUY alta frecuencia ................................ no compartir: fragmentar . caso 2.3
│
├─ Pasar datos entre goroutines
│    ├─ productor → consumidor ............................. chan (+ close, range) .... caso 3.1
│    ├─ varias etapas ...................................... pipeline + ctx ........... caso 3.2
│    └─ repartir entre N y conservar el orden .............. fan-out/fan-in .......... caso 3.3
│
├─ Plazos, cancelación, fallos transitorios
│    ├─ no esperar para siempre ............................ context.WithTimeout ...... caso 4.1
│    ├─ reintentar ......................................... backoff + jitter ......... caso 4.2
│    └─ apagar sin perder trabajo .......................... close + gracia + ctx ..... caso 4.3
│
├─ Proteger al sistema / a un servicio
│    ├─ máximo N a la vez .................................. semáforo ................. caso 5.1
│    ├─ cola acotada (bloquear o rechazar) ................. worker pool .............. caso 5.2
│    └─ máximo X por segundo y por cliente ................. rate.Limiter (429) ....... caso 5.3
│
└─ Ir más rápido con CPU
     ├─ cálculo repartible entre núcleos ................... particionar + local ...... caso 6.1 / 6.2
     ├─ divide y vencerás ................................... recursión + umbral ....... caso 6.3
     ├─ ¿cuántas goroutines? ................................ E/S: muchas · CPU: ≈núcleos  caso 7.1
     └─ ya no cabe en una máquina ........................... contenedores ............. caso 7.2
```

## Features de Go: cuándo usarlas y cuándo no

| Feature | Úsala cuando… | No la uses cuando… | Caso | Módulo |
|--|--|--|--|--|
| `go f()` | la tarea es independiente y espera o calcula | necesitas saber cuándo acaba y no lo gestionas (fuga) | [1.1](01-lanzar-y-esperar/1-basico-waitgroup) | [02-goroutines](../02-goroutines) |
| `sync.WaitGroup.Go` | lanzar N y esperar a todas | necesitas errores o cancelación | [1.1](01-lanzar-y-esperar/1-basico-waitgroup) | [02-goroutines/03-waitgroup](../02-goroutines/03-waitgroup) |
| `errgroup` | lote «todo o nada», límite de concurrencia | quieres **todos** los errores (usa `errors.Join`) | [1.2](01-lanzar-y-esperar/2-intermedio-errgroup) | [10-errores/01-errgroup](../10-errores/01-errgroup) |
| `sync.Mutex` | proteger una estructura / varias variables | un solo número (atomic) o un flujo de datos (channel) | [2.1](02-estado-compartido/1-basico-mutex) | [06-sync/01-mutex](../06-sync/01-mutex) |
| `sync/atomic` | un contador, bandera o puntero | dos variables que cambian juntas | [2.1](02-estado-compartido/1-basico-mutex) | [07-atomic](../07-atomic) |
| `sync.RWMutex` | lecturas largas ≫ escrituras | secciones críticas muy cortas (un `Mutex` puede ganar) | [2.2](02-estado-compartido/2-intermedio-cache) | [06-sync/02-rwmutex](../06-sync/02-rwmutex) |
| `singleflight` | evitar la estampida en una caché/consulta cara | el resultado depende del llamante | [2.2](02-estado-compartido/2-intermedio-cache) | [10-errores/04-singleflight](../10-errores/04-singleflight) |
| `sync.Once` / `OnceValue` | inicialización única y perezosa | la inicialización puede fallar y reintentarse | — | [06-sync/03-once](../06-sync/03-once) |
| `sync.Pool` | reutilizar objetos temporales (menos GC) | guardar estado con significado (el GC lo vacía) | — | [06-sync/06-pool](../06-sync/06-pool) |
| `sync.Map` | claves escritas una vez / conjuntos disjuntos | un mapa normal con `Mutex` (más claro, tipado) | — | [06-sync/05-sync-map](../06-sync/05-sync-map) |
| `sync.Cond` | casi nunca: esperar una condición sobre estado compartido | ya tienes channels (suelen bastar y son cancelables) | — | [06-sync/04-cond](../06-sync/04-cond) |
| `chan T` | transferir datos / sincronizar | proteger estado compartido simple | [3.1](03-comunicar-datos/1-basico-productor-consumidor) | [03-channels](../03-channels) |
| `chan` con buffer | absorber ráfagas / acotar una cola | «para que no se bloquee» sin pensar el tamaño | [3.1](03-comunicar-datos/1-basico-productor-consumidor) · [5.2](05-limitar-recursos/2-intermedio-worker-pool) | [03-channels/01](../03-channels/01-sin-buffer-vs-buffer) |
| `select` | esperar varias cosas a la vez, plazos | un solo origen (use `<-ch` directo) | [4.1](04-tiempo-y-cancelacion/1-basico-timeout) | [04-select-timers](../04-select-timers) |
| `context` | propagar plazos y cancelación por toda la pila | pasar datos opcionales (no abuses de `WithValue`) | [4.1](04-tiempo-y-cancelacion/1-basico-timeout) | [05-context](../05-context) |
| `time.Timer`/`Ticker` | esperas cancelables, tareas periódicas | `time.Sleep` donde haya que poder cancelar | [4.2](04-tiempo-y-cancelacion/2-intermedio-reintentos) | [04-select-timers/04-ticker](../04-select-timers/04-ticker) |
| `rate.Limiter` | limitar tasa por cliente / respetar una cuota ajena | limitar concurrencia (semáforo) | [5.3](05-limitar-recursos/3-avanzado-rate-limit) | [08-patrones/06-rate-limiting](../08-patrones/06-rate-limiting) |
| `semaphore.Weighted` | recursos con peso (memoria, conexiones) | todas las tareas pesan lo mismo (un `chan` basta) | [5.1](05-limitar-recursos/1-basico-semaforo) | [08-patrones/05-semaforo](../08-patrones/05-semaforo) |
| `iter.Seq` | recorrer una secuencia **sin** concurrencia | necesitas productor concurrente (channel) | — | [09-iteradores](../09-iteradores) |
| `testing/synctest` | probar código con tiempo/concurrencia de forma determinista | código con E/S de red real o mutex bloqueados | todos | [11-testing](../11-testing) |
| `pprof` / `trace` | medir **antes** de optimizar | adivinar | — | [12-profiling](../12-profiling) |

## Ruta de estudio sugerida

Intercalando esta carpeta con los módulos (el módulo da la *herramienta*; el caso, el *cuándo*):

| Paso | Módulo(s) | Después, estos casos |
|--|--|--|
| 1 | [01-fundamentos](../01-fundamentos), [02-goroutines](../02-goroutines) | 1.1, 7.1 |
| 2 | [03-channels](../03-channels) | 3.1 |
| 3 | [04-select-timers](../04-select-timers), [05-context](../05-context) | 4.1 |
| 4 | [06-sync](../06-sync), [07-atomic](../07-atomic) | 2.1, 2.2 |
| 5 | [08-patrones](../08-patrones) | 3.2, 5.1, 5.2 |
| 6 | [09-iteradores](../09-iteradores), [10-errores](../10-errores) | 1.2, 3.3, 4.2 |
| 7 | [11-testing](../11-testing) | repite cualquier test con `-race -count=5` y entiende `synctest` |
| 8 | [12-profiling](../12-profiling), [13-rendimiento](../13-rendimiento) | 2.3, 6.1, 6.2, 6.3 |
| 9 | [14-internals](../14-internals) | revisa 7.1 con lo que sabes de G-M-P y del netpoller |
| 10 | [15-diseno-avanzado](../15-diseno-avanzado) | 1.3, 4.3, 5.3 |
| 11 | [proyecto-final](../proyecto-final) | 7.2 y todo junto |

## Los diez errores más comunes (y dónde se ven)

| # | Error | Consecuencia | Dónde |
|--|--|--|--|
| 1 | no esperar a las goroutines (`Wait`) | el programa termina a medias | [1.1](01-lanzar-y-esperar/1-basico-waitgroup) |
| 2 | variable compartida sin proteger | carrera de datos: resultados erróneos que no se repiten | [2.1](02-estado-compartido/1-basico-mutex) |
| 3 | goroutine sin condición de salida | **fuga** de goroutines y de memoria | [4.1](04-tiempo-y-cancelacion/1-basico-timeout), [3.2](03-comunicar-datos/2-intermedio-pipeline) |
| 4 | enviar en un channel que nadie lee | goroutine bloqueada para siempre | [3.2](03-comunicar-datos/2-intermedio-pipeline) |
| 5 | olvidar `close` / cerrar desde el receptor / cerrar dos veces | interbloqueo o pánico | [3.1](03-comunicar-datos/1-basico-productor-consumidor), [4.3](04-tiempo-y-cancelacion/3-avanzado-apagado-ordenado) |
| 6 | operaciones sin plazo (`context.Background()`) | esperas indefinidas | [4.1](04-tiempo-y-cancelacion/1-basico-timeout) |
| 7 | reintentar sin backoff ni jitter | convierte un fallo parcial en caída total | [4.2](04-tiempo-y-cancelacion/2-intermedio-reintentos) |
| 8 | cola o concurrencia sin límite | sobrecarga, latencia enorme, agotar recursos | [5.1](05-limitar-recursos/1-basico-semaforo), [5.2](05-limitar-recursos/2-intermedio-worker-pool) |
| 9 | un mutex global «para ir rápido en paralelo» | **más lento que el secuencial** (9.8× en 6.2) | [6.2](06-paralelismo-cpu/2-intermedio-map-reduce) |
| 10 | paralelizar sin medir / mal dimensionar el pool | más goroutines ≠ más rápido | [6.1](06-paralelismo-cpu/1-basico-suma-paralela), [7.1](07-io-vs-cpu/1-basico-cuantos-workers) |

## Cuatro ideas para llevarte

1. **Primero decide qué limita tu programa**: ¿espera (E/S), calcula (CPU), toca memoria o pelea por un cerrojo? De eso depende todo lo demás (7.1, módulo 13).
2. **La mejor sincronización es la que no existe**: resultado por índice, estado local + fusionar, pasar el dato en vez de compartirlo (1.1, 2.3, 6.2).
3. **Toda goroutine necesita una condición de salida y un dueño**: `ctx`, `close` o `Wait` (4.1, 4.3, módulo 15).
4. **Mide antes y después**: los casos 2.3, 6.1-6.3 y 7.1 existen para que veas con números dónde compensa y dónde no.

## Convenciones y ejecución

- Cada caso es un `package main` independiente con `main.go` (comentado con el *porqué*), `main_test.go` y `README.md`.
- Los tests usan `-race` y, cuando hay tiempo de por medio, `testing/synctest` (reloj virtual: instantes **exactos** y sin esperas reales) y `goleak` (cero goroutines al terminar).
- Todos los resultados de los README salen de ejecutar los programas en un Apple M1 (8 núcleos, Go 1.26.5); las cifras absolutas cambian con tu máquina, **lo reutilizable es la forma** (qué escala, qué no, y por qué).

```bash
go vet ./casos-de-uso/...
go test -race -count=1 ./casos-de-uso/...
```
