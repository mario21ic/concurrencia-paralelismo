# Módulo 2. Goroutines

**Objetivo:** lanzar y coordinar goroutines correctamente, y entender cómo las planifica el runtime.

## Ejercicios

| # | Carpeta | Tema |
|---|---------|------|
| 1 | [01-go-basico](01-go-basico) | La palabra clave `go`; `main` no espera a nadie |
| 2 | [02-variables-de-bucle](02-variables-de-bucle) | Captura de variables de bucle (semántica 1.22) |
| 3 | [03-waitgroup](03-waitgroup) | `sync.WaitGroup`, `WaitGroup.Go` (1.25) y errores típicos |
| 4 | [04-runtime-info](04-runtime-info) | `NumCPU`, `GOMAXPROCS`, `NumGoroutine`, stack dinámico |
| 5 | [05-scheduler](05-scheduler) | Modelo G-M-P, work stealing, preemption |
| 6 | [06-urls-concurrentes](06-urls-concurrentes) | Ejercicio: consultar N URLs en paralelo |
| 7 | [07-arreglar-race](07-arreglar-race) | Ejercicio: reproducir y arreglar un data race |

```bash
cd 02-goroutines/01-go-basico && go run .
```

> Tiempos de los README: Mac de 8 núcleos, Go 1.26.5. Ejecuta con `-race` siempre que puedas.

## Resumen teórico

- `go f(x)` evalúa `f` y los argumentos **ahora**, y ejecuta `f` en una nueva goroutine **cuando el scheduler decida**. El valor de retorno se descarta.
- Cuando `main` retorna, el programa termina sin esperar a nadie.
- **Una goroutine siempre debe tener una forma clara de terminar** y alguien que sepa que terminó.
- `sync.WaitGroup` espera a un grupo; desde Go 1.25, `wg.Go(f)` hace `Add(1)` + `go` + `defer Done()` en un paso.
- Desde Go 1.22 cada iteración de `for` tiene su propia variable, lo que elimina el bug clásico de closures.
- Un panic no recuperado en **cualquier** goroutine termina todo el proceso.
- El scheduler es **M:N** y tiene tres piezas: **G** (goroutine), **M** (hilo de SO) y **P** (contexto/permiso para ejecutar Go; hay `GOMAXPROCS`). Cada P tiene una cola local; si se queda vacía, roba de otro P (*work stealing*) o de la cola global.
- Desde Go 1.14 la preemption es asíncrona (basada en señales): un bucle apretado ya no bloquea a las demás.
- Desde Go 1.25 `GOMAXPROCS` por defecto respeta el límite de CPU del cgroup (contenedores/Kubernetes), no solo el número de núcleos de la máquina.

## Preguntas de repaso

1. ¿Qué imprime un programa cuyo `main` hace `go fmt.Println("hola")` y retorna? ¿Siempre?
2. ¿Por qué `time.Sleep` no es un mecanismo válido de sincronización?
3. ¿Qué pasa si llamas a `wg.Add(1)` dentro de la goroutine?
4. ¿Qué es un P y qué ocurre con él cuando una goroutine se bloquea en una llamada al sistema?
5. ¿Qué diferencia hay entre `GOMAXPROCS` y `NumCPU`?

## Lecturas

- [Go 1.22: loopvar](https://go.dev/blog/loopvar-preview)
- Documentación de [`sync.WaitGroup`](https://pkg.go.dev/sync#WaitGroup) y [`runtime`](https://pkg.go.dev/runtime)
- Go Scheduler design doc (Dmitry Vyukov) — *Scalable Go Scheduler Design Doc*
