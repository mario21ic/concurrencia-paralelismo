# Sílabo: Concurrencia y Paralelismo en Go 1.26

Curso autodidacta de nivel básico a avanzado. Cada módulo incluye objetivos, temas, ejercicios prácticos y un mini-proyecto.

- **Duración estimada:** 10–12 semanas (6–8 h/semana)
- **Requisitos:** Go 1.26 instalado, conocimientos básicos de Go (tipos, structs, interfaces, funciones, slices y maps, manejo de errores)
- **Verificar entorno:** `go version` → `go1.26.x`

## Estructura sugerida del repositorio

```
.
├── README.md
├── 01-fundamentos/
├── 02-goroutines/
├── 03-channels/
├── 04-select-context/
├── 05-sync/
├── 06-patrones/
├── 07-errores-cancelacion/
├── 08-testing/
├── 09-rendimiento/
├── 10-avanzado/
└── proyectos/
```

Cada carpeta con su propio `main.go` / `*_test.go`. Ejecuta **siempre** con `-race` mientras aprendes:

```bash
go run -race .
go test -race ./...
```

---

## Nivel 1 — Básico

### Módulo 1. Fundamentos teóricos
**Objetivos:** distinguir concurrencia de paralelismo y entender el modelo de Go.

- Concurrencia vs paralelismo (Rob Pike: *"Concurrency is not parallelism"*)
- CPU-bound vs I/O-bound
- Procesos, hilos de SO e hilos "verdes"
- Modelo CSP: *"Don't communicate by sharing memory; share memory by communicating"*
- Ley de Amdahl y límites del paralelismo
- Memory model de Go y relación *happens-before*

**Ejercicio:** diagrama propio de un programa secuencial vs concurrente vs paralelo.

### Módulo 2. Goroutines
**Objetivos:** lanzar y coordinar goroutines correctamente.

- La palabra clave `go` y el costo de una goroutine (stack inicial pequeño, crece dinámicamente)
- El programa termina cuando `main` termina
- Captura de variables de bucle (semántica por iteración desde Go 1.22)
- `sync.WaitGroup` y **`WaitGroup.Go`** (Go 1.25+)
- `runtime.GOMAXPROCS`, `runtime.NumCPU`, `runtime.NumGoroutine`
- GOMAXPROCS consciente de contenedores/cgroups (Go 1.25+)
- Scheduler M:N: modelo **G-M-P**, work stealing, preemption asíncrona

**Ejercicios:**
1. Lanzar 10 000 goroutines y medir memoria.
2. Descargar N URLs en paralelo con `WaitGroup`.
3. Reproducir y arreglar un *data race* detectado por `-race`.

### Módulo 3. Channels
**Objetivos:** comunicar goroutines de forma segura.

- Channels sin buffer vs con buffer (semántica de sincronización)
- Direccionalidad: `chan<-`, `<-chan`
- `close`, `range` sobre channel, idioma `v, ok := <-ch`
- Channels `nil` (bloquean siempre; útiles en `select`)
- Deadlocks: "all goroutines are asleep"
- Reglas: quién cierra el channel (el emisor, nunca el receptor)
- Panics comunes: enviar a channel cerrado, cerrar dos veces

**Ejercicios:**
1. Ping-pong entre dos goroutines.
2. Generador de números con channel.
3. Provocar a propósito cada panic/deadlock y explicar por qué ocurre.

**Mini-proyecto 1:** contador de palabras concurrente sobre varios archivos.

---

## Nivel 2 — Intermedio

### Módulo 4. `select` y temporizadores
- `select` con múltiples casos, `default` y selección aleatoria
- Timeouts con `time.After`, `time.NewTimer`, `time.Ticker`
- Cambios en timers desde Go 1.23 (GC de timers sin `Stop`, channels sin buffer efectivo)
- Patrón *non-blocking send/receive*
- Desactivar casos con channels `nil`

**Ejercicio:** consulta a 3 servidores simulados y devolver la respuesta más rápida.

### Módulo 5. `context`
- `context.Background`, `WithCancel`, `WithTimeout`, `WithDeadline`
- `WithCancelCause` y `context.Cause`
- `context.AfterFunc` y `WithoutCancel`
- `context.WithValue`: cuándo sí y cuándo no
- Propagación: `ctx` como primer parámetro
- Cancelación cooperativa: revisar `ctx.Done()` en bucles largos

**Ejercicio:** worker que se detiene limpiamente con `Ctrl+C` (`signal.NotifyContext`).

### Módulo 6. Paquete `sync`
- `sync.Mutex` y `sync.RWMutex` (cuándo RWMutex *no* ayuda)
- `sync.Once`, `OnceFunc`, `OnceValue`, `OnceValues`
- `sync.Cond` (y por qué casi siempre hay alternativas)
- `sync.Map` (casos de uso reales y limitaciones)
- `sync.Pool` (reducir presión sobre el GC)
- Composición: proteger invariantes, no variables sueltas
- Buenas prácticas: `defer mu.Unlock()`, no copiar mutex, secciones críticas cortas

### Módulo 7. Paquete `sync/atomic`
- Operaciones atómicas: `Add`, `Load`, `Store`, `CompareAndSwap`
- Tipos genéricos: `atomic.Int64`, `atomic.Bool`, `atomic.Pointer[T]`, `atomic.Value`
- Operaciones `And`/`Or` (Go 1.23+)
- Cuándo atomics vs mutex vs channel
- Estructuras *lock-free* simples (contador, flag, config recargable)

**Ejercicio:** benchmark de contador con `Mutex` vs `atomic` vs channel.

**Mini-proyecto 2:** caché en memoria thread-safe con TTL y expiración por `context`.

---

## Nivel 3 — Patrones de concurrencia

### Módulo 8. Patrones clásicos
- **Worker pool** (tamaño fijo, trabajos por channel)
- **Fan-out / Fan-in**
- **Pipeline** por etapas, con cancelación
- **Productor–consumidor**
- **Semáforo** con channel con buffer o `golang.org/x/sync/semaphore`
- **Rate limiting** (`time.Ticker`, token bucket, `x/time/rate`)
- **Pub/Sub** y broadcast
- **Or-done channel**, **tee channel**, **bridge channel**

### Módulo 9. Iteradores y concurrencia (Go 1.23+)
- `iter.Seq`, `iter.Seq2` y `range over func`
- `iter.Pull` y su relación con corrutinas
- Pipelines con iteradores vs channels: cuándo cada uno

### Módulo 10. Manejo de errores y cancelación
- `golang.org/x/sync/errgroup`: `Group`, `WithContext`, `SetLimit`
- `errors.Join` para agregar errores de múltiples goroutines
- Recuperar `panic` dentro de goroutines (un panic sin recover tumba todo el proceso)
- Propagar errores en pipelines
- `singleflight` para deduplicar llamadas idénticas
- Apagado ordenado (*graceful shutdown*) de servicios

**Mini-proyecto 3:** crawler web concurrente con límite de concurrencia, rate limit, timeouts y cancelación.

---

## Nivel 4 — Avanzado

### Módulo 11. Testing de código concurrente
- `go test -race`, `-count`, `-shuffle`, `-cpu`
- Tests deterministas sin `time.Sleep`
- **`testing/synctest`** (estable desde Go 1.25): "burbuja" con reloj falso, `synctest.Test` y `synctest.Wait`
- Tests de timeouts y reintentos en tiempo virtual
- Detectar *goroutine leaks* (`goleak`, y el perfil `goroutineleak` experimental en Go 1.26)
- *Stress testing* y fuzzing de código concurrente

### Módulo 12. Profiling y observabilidad
- `pprof`: perfiles `goroutine`, `mutex`, `block`, `cpu`, `heap`
- `runtime/trace` y `go tool trace`
- **Flight recorder** de `runtime/trace` (Go 1.25+)
- Métricas con `runtime/metrics`
- Diagnóstico de contención, goroutines bloqueadas y *leaks*
- `GODEBUG` y variables relevantes (`GOMAXPROCS`, `GOGC`, `GOMEMLIMIT`)

### Módulo 13. Rendimiento y paralelismo real
- Escalabilidad con núcleos: medir, no suponer
- *False sharing* y alineación de cache lines (`padding`)
- Reducir contención: sharding de locks, estructuras por-goroutine
- Batching y reducción de asignaciones
- Benchmarks con `testing.B` y `b.Loop()`, paralelos con `b.RunParallel`
- Comparar con `benchstat`
- Recolector de basura **Green Tea** (activo por defecto en Go 1.26) y su efecto en programas con muchas goroutines y objetos pequeños
- Paralelismo de datos: *map-reduce*, particionar slices, *parallel merge sort*

### Módulo 14. Internals del runtime
- Scheduler G-M-P en detalle: colas locales y globales, *work stealing*, *netpoller*
- Llamadas bloqueantes al sistema y `handoff` de P
- Preemption cooperativa vs asíncrona
- `runtime.Gosched`, `runtime.LockOSThread`, cuándo importa (cgo, GUI, OpenGL)
- Crecimiento de stacks de goroutines
- Cómo se implementan channels (`hchan`), `select` y mutex internamente
- Modo *starvation* de `sync.Mutex`

### Módulo 15. Diseño avanzado y problemas clásicos
- Deadlock, livelock, starvation, *priority inversion*
- Problemas clásicos: filósofos comensales, lectores–escritores, barbero dormilón
- Estructuras concurrentes: cola acotada, ring buffer, skip list concurrente, hash map con sharding
- Lock-free y wait-free: ABA, CAS loops, límites de la técnica
- Diseño de APIs concurrentes: ¿quién lanza la goroutine, quién la detiene?
- Concurrencia estructurada en Go: ciclo de vida atado a `ctx` + `errgroup`/`WaitGroup.Go`
- Concurrencia en servidores `net/http`: goroutine por request, límites y backpressure
- Concurrencia y bases de datos: `database/sql` pool, transacciones y cancelación

---

## Proyecto final (elige uno)

1. **Servidor de chat en tiempo real** con salas, broadcast, backpressure y apagado ordenado.
2. **Job scheduler / cola de tareas** con prioridades, reintentos, rate limiting y métricas `pprof`.
3. **Pipeline de procesamiento de logs** (lectura → parseo → agregación → salida) con workers dinámicos.
4. **Mini-scraper distribuido** con cola, deduplicación (`singleflight`) y respeto de `robots.txt`.

**Criterios de evaluación:**
- Pasa `go test -race ./...` sin fallos
- Sin goroutine leaks (verificado con `synctest`/`goleak`)
- Cancelación y apagado ordenado vía `context`
- Benchmarks y perfil `pprof`/`trace` documentados
- README con decisiones de diseño y diagrama

---

## Cronograma sugerido

| Semana | Módulos | Entregable |
|-------:|---------|------------|
| 1 | 1–2 | Ejercicios de goroutines y `-race` |
| 2 | 3 | Mini-proyecto 1 |
| 3 | 4–5 | Worker con cancelación por señal |
| 4–5 | 6–7 | Mini-proyecto 2 (caché) |
| 6–7 | 8–10 | Mini-proyecto 3 (crawler) |
| 8 | 11 | Suite de tests con `synctest` |
| 9 | 12–13 | Informe de profiling y benchmarks |
| 10 | 14–15 | Implementar estructura concurrente propia |
| 11–12 | — | Proyecto final |

---

## Novedades de Go relevantes para concurrencia

| Versión | Novedad |
|---------|---------|
| 1.22 | Variable de bucle por iteración (fin del bug clásico con closures) |
| 1.23 | Iteradores (`iter`), timers recolectables, `atomic.And/Or` |
| 1.24 | `testing/synctest` experimental, `b.Loop()`, `weak` pointers |
| 1.25 | `sync.WaitGroup.Go`, `testing/synctest` estable, GOMAXPROCS consciente de cgroups, flight recorder |
| 1.26 | Recolector Green Tea por defecto, perfil experimental de goroutine leaks |

> Verifica los detalles en las notas oficiales de cada versión: <https://go.dev/doc/devel/release>

---

## Recursos recomendados

**Oficiales**
- A Tour of Go — <https://go.dev/tour>
- The Go Memory Model — <https://go.dev/ref/mem>
- Effective Go — <https://go.dev/doc/effective_go>
- Blog de Go: *Go Concurrency Patterns*, *Pipelines and cancellation*, *Context*, *Testing concurrent code with testing/synctest*
- Documentación de `sync`, `sync/atomic`, `context`, `testing/synctest`

**Libros**
- *Concurrency in Go* — Katherine Cox-Buday
- *100 Go Mistakes and How to Avoid Them* — Teiva Harsanyi (capítulos de concurrencia)
- *The Go Programming Language* — Donovan & Kernighan (caps. 8 y 9)
- *Learning Go* (2.ª ed.) — Jon Bodner

**Charlas**
- *Concurrency Is Not Parallelism* — Rob Pike
- *Go Concurrency Patterns* — Rob Pike
- *Advanced Go Concurrency Patterns* — Sameer Ajmani
- *Understanding Channels* — Kavya Joshi
- *The Scheduler Saga* — Kavya Joshi

**Herramientas**
- `go vet`, `staticcheck`, `golangci-lint`
- `go tool pprof`, `go tool trace`
- `benchstat`, `go.uber.org/goleak`
- `golang.org/x/sync` (`errgroup`, `semaphore`, `singleflight`)

---

## Checklist de dominio

- [ ] Explico la diferencia entre concurrencia y paralelismo
- [ ] Sé cuándo usar channel, mutex o atomic
- [ ] Nunca lanzo una goroutine sin saber cómo y cuándo termina
- [ ] Propago `context` y respeto la cancelación
- [ ] Escribo tests concurrentes deterministas con `synctest`
- [ ] Diagnostico leaks y contención con `pprof` y `trace`
- [ ] Puedo explicar el scheduler G-M-P
- [ ] Mido antes de optimizar
