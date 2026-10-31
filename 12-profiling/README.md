# Módulo 12. Profiling y observabilidad

**Objetivo:** diagnosticar con **datos** (no con intuición) los problemas de rendimiento y de concurrencia de un programa Go: CPU, contención, bloqueos, fugas, latencia del planificador y recolector de basura, tanto en desarrollo como en un servicio en producción.

## Ejercicios

| # | Carpeta | Tema |
|---|---------|------|
| 1 | [01-pprof-cpu](01-pprof-cpu) | Perfil de CPU: `-top`, `-cum`, `-list`; reducir asignaciones |
| 2 | [02-mutex-y-block](02-mutex-y-block) | Perfiles de contención y de bloqueo; fragmentar un mutex |
| 3 | [03-goroutines-bloqueadas](03-goroutines-bloqueadas) | Perfil de goroutines: estados, `debug=1/2`, caza de fugas |
| 4 | [04-trace](04-trace) | `runtime/trace`: tareas, regiones, logs; `go tool trace -pprof` |
| 5 | [05-flight-recorder](05-flight-recorder) | *Flight recorder*: capturar el pasado reciente de un incidente |
| 6 | [06-runtime-metrics](06-runtime-metrics) | `runtime/metrics`: planificador, mutex, CPU; histogramas |
| 7 | [07-gc-y-entorno](07-gc-y-entorno) | `GOGC`, `GOMEMLIMIT`, `gctrace`, Green Tea frente al GC clásico |
| 8 | [08-diagnostico](08-diagnostico) | **Caso integrador:** un servicio con 3 problemas ocultos |

Además, `internal/traza` es un **analizador de trazas** reutilizable (con `golang.org/x/exp/trace`) que resume una traza en el terminal: tiempo por estado, esperas por motivo, regiones y tareas.

```bash
cd 12-profiling/01-pprof-cpu && go run . -modo=lento -perfil lento.pprof && go tool pprof -top lento.pprof
```

> Mediciones: Mac M1 de 8 núcleos, Go 1.26.5. Los números absolutos cambian de una máquina a otra y, en cargas extremas, entre ejecuciones; fíjate en la **dirección** y el **orden de magnitud**.

## Resumen teórico

### Qué herramienta para qué síntoma

| Síntoma | Herramienta | Ejercicio |
|--|--|--|
| CPU al 100 % | **perfil de CPU** (`-top`, `-list`) | 1, 8 |
| CPU libre pero latencia alta | **perfil mutex / block**, **traza** | 2, 4 |
| "Está colgado" / goroutines que no avanzan | **perfil goroutine** (`debug=2`), `SIGQUIT` | 3 |
| Goroutines o memoria que crecen | perfil goroutine `debug=1` (comparar en el tiempo), perfil de heap | 3, 8 |
| ¿Se aprovechan los núcleos? ¿cuánto esperan las goroutines? | **traza** | 4 |
| Latencia de cola (*p99*) esporádica | **flight recorder** | 5 |
| Vigilar el servicio de forma continua | **`runtime/metrics`** | 6 |
| Mucha CPU en GC / memoria que se dispara | `GODEBUG=gctrace=1`, `GOGC`, `GOMEMLIMIT` | 7 |

### Los perfiles de `runtime/pprof`

| Perfil | Muestrea | Activación | Mide |
|--|--|--|--|
| `cpu` | ~100 Hz, `SIGPROF` | `StartCPUProfile` / `?seconds=N` | dónde se **ejecuta** |
| `heap` / `allocs` | cada ~512 KB asignados | siempre activo | quién **asigna** (`-alloc_space`, `-inuse_space`) |
| `goroutine` | foto completa | siempre activo | dónde están **todas** las goroutines ahora |
| `mutex` | eventos de contención | `SetMutexProfileFraction(n)` | quién **retiene** locks que bloquean a otros |
| `block` | bloqueos de sincronización | `SetBlockProfileRate(ns)` | dónde **esperan** las goroutines |
| `threadcreate` | creación de hilos | siempre | uso de hilos del SO |
| `goroutineleak` *(experimental, 1.26)* | recolección | `GOEXPERIMENT=goroutineleakprofile` | goroutines **inalcanzables** (fugas seguras) |

### Cheat sheet de `go tool pprof`

```bash
go tool pprof -top  perfil              # resumen por flat
go tool pprof -top -cum perfil          # por tiempo acumulado
go tool pprof -list 'regex' perfil      # por línea de fuente
go tool pprof -peek 'regex' perfil      # quién llama / a quién llama
go tool pprof -focus=regex -top perfil  # solo caminos que pasan por regex
go tool pprof -base a.pb.gz b.pb.gz     # diferencia entre dos perfiles (¿qué creció?)
go tool pprof -http=:8081 perfil        # interfaz web: grafo, llamas (flame graph), fuente
go tool pprof -sample_index=contentions perfil   # otro tipo de muestra
go tool pprof URL                       # perfil en caliente de un servicio (guarda en ~/pprof/)
```

*Flat* = en la propia función; *cum* = en ella y en lo que llama.

### Trazas

- **Perfil** = estadística (muestras). **Traza** = **todos los eventos** con tiempo exacto: más detalle, más coste y más datos.
- `go tool trace`: visor web; `-pprof=sync|sched|net|syscall`: perfiles derivados.
- Anota con `trace.NewTask`, `trace.WithRegion` y `trace.Log` para dar sentido de negocio.
- **Flight recorder** (Go 1.25): ventana móvil en memoria; se vuelca solo ante un incidente.

### Métricas (`runtime/metrics`)

Contadores, medidores e histogramas **estables** y baratos: `/sched/goroutines…`, `/sched/latencies:seconds`, `/sync/mutex/wait/total:seconds`, `/cpu/classes/*`, `/gc/*`, `/memory/classes/*`. Los contadores se interpretan como **tasas**; los histogramas se **restan** entre dos fotos.

### Variables de entorno y `GODEBUG`

| Variable | Efecto |
|--|--|
| `GOMAXPROCS` | nº de `P` (desde Go 1.25 respeta el límite de CPU del contenedor) |
| `GOGC` | proporción de crecimiento del heap entre ciclos (100) |
| `GOMEMLIMIT` | límite **blando** de memoria |
| `GODEBUG=gctrace=1` | una línea por ciclo de GC |
| `GODEBUG=schedtrace=1000` | estado del planificador cada 1000 ms (módulo 2) |
| `GOTRACEBACK=all` | pilas de todas las goroutines en un pánico |
| `GOEXPERIMENT=nogreenteagc` | recolector clásico (al compilar) |

### Reglas de oro

1. **Mide primero, optimiza después.** Un perfil dice dónde está el tiempo; la intuición suele equivocarse.
2. **Cambia una cosa y vuelve a medir** con la misma carga y la misma herramienta.
3. **Mide en condiciones realistas** (carga real, datos reales, hardware real): un microbenchmark no ve la contención ni la presión del GC.
4. **Cada perfil tiene un punto ciego** (el de CPU no ve esperas; el de bloqueo mezcla esperas inocentes). Combínalos.
5. **Los perfiles tienen coste**: activa mutex/block con fracciones altas en producción; la traza completa solo por ratos; usa el flight recorder para lo continuo.
6. **Protege `net/http/pprof`:** nunca en un puerto público; puerto interno o con autenticación.
7. **Observabilidad continua:** exporta `goroutines`, `runnable`, latencia de planificación, espera de mutex y GC como métricas con alertas, para enterarte *antes* de abrir un perfil.

## Hallazgos del módulo (aprendidos midiendo)

- **Menos asignaciones escalan mejor que menos instrucciones:** 2.9× en un hilo, 5.1× con 8 workers (ejercicio 1).
- **Fragmentar un mutex** redujo la espera 8.7× y el tiempo 4.5× (ejercicio 2).
- **Una fuga no se ve en la latencia ni en el rendimiento**, solo en el recuento de goroutines (ejercicio 8).
- **`GOGC` alto** reduce el GC del 48 % al 16 % de la CPU, pero multiplica el pico de heap por 20 (ejercicio 7).
- **Saturar el planificador** (32× goroutines que núcleos) deja la CPU al 100 % pero multiplica la latencia de planificación por ~10⁵ (ejercicio 6).
- Dos errores míos que las herramientas destaparon: una **carrera de datos** en el código de benchmark (`-race`, ejercicio 2) y una carga de GC mal diseñada cuyo pico de heap de GiB lo delató (ejercicio 7).

## Preguntas de repaso

1. ¿Qué significa que un perfil de CPU tenga `Total samples = 2800ms (398%)` en una ejecución de 700 ms?
2. ¿Qué diferencia hay entre *flat* y *cum*? ¿Cuándo mirar cada uno?
3. ¿Por qué un perfil de CPU no encuentra un problema de contención de un mutex?
4. ¿Qué atribuye el perfil de *mutex* y qué el de *block*? ¿Qué pistas falsas aparecen en el de *block*?
5. ¿Cómo distingues una fuga de goroutines de goroutines legítimamente bloqueadas?
6. ¿Qué ve una traza que no vea un perfil? ¿Cuál es su coste?
7. ¿Qué problema resuelve el *flight recorder* y cómo se usa sin saturar el disco?
8. ¿Por qué `runnable` y la latencia de planificación detectan saturación y la "CPU al 100 %" no?
9. ¿Qué compromiso hace `GOGC`? ¿Y `GOMEMLIMIT`? ¿Por qué mantener `GOGC` activo con `GOMEMLIMIT`?
10. ¿Por qué un `runtime.GC()` forzado contamina una medición de latencia del planificador?

## Lecturas

- [Profiling Go Programs](https://go.dev/blog/pprof) y [Diagnostics](https://go.dev/doc/diagnostics)
- [A Guide to the Go Garbage Collector](https://go.dev/doc/gc-guide)
- [Flight Recorder in Go 1.25](https://go.dev/blog/flight-recorder)
- Documentación de [`runtime/pprof`](https://pkg.go.dev/runtime/pprof), [`runtime/trace`](https://pkg.go.dev/runtime/trace) y [`runtime/metrics`](https://pkg.go.dev/runtime/metrics)
- [`golang.org/x/exp/trace`](https://pkg.go.dev/golang.org/x/exp/trace) (lector programático de trazas)
- *Systems Performance* — Brendan Gregg (método USE, flame graphs)
