# Módulo 8. Patrones clásicos de concurrencia

**Objetivo:** dominar los bloques de construcción con los que se componen programas concurrentes reales, en versiones **genéricas, cancelables y sin fugas**.

## Ejercicios

| # | Carpeta | Patrón |
|---|---------|--------|
| 1 | [01-worker-pool](01-worker-pool) | Worker pool de tamaño fijo |
| 2 | [02-fan-out-fan-in](02-fan-out-fan-in) | Fan-out / fan-in |
| 3 | [03-pipeline](03-pipeline) | Pipeline por etapas con cancelación |
| 4 | [04-productor-consumidor](04-productor-consumidor) | Productor–consumidor, contrapresión y batching |
| 5 | [05-semaforo](05-semaforo) | Semáforo (channel y `x/sync/semaphore`) |
| 6 | [06-rate-limiting](06-rate-limiting) | Ticker, token bucket y `x/time/rate` |
| 7 | [07-pubsub](07-pubsub) | Publicador/suscriptor y broadcast |
| 8 | [08-or-done-tee-bridge](08-or-done-tee-bridge) | `OrDone`, `Tee`, `Bridge` |

```bash
cd 08-patrones/01-worker-pool && go run . && go test -race -count=3 .
```

> Los ejercicios 5 y 6 usan `golang.org/x/sync` y `golang.org/x/time` (ya declarados en el `go.mod` de la raíz).

## Resumen teórico

### Mapa de patrones

```
Worker pool            jobs ──► [w][w][w][w] ──► resultados      (paralelismo acotado)
Fan-out                un origen ──► N workers (cada valor lo recibe UNO)
Fan-in                 N orígenes ──► un solo channel
Pipeline               etapa1 ──► etapa2 ──► etapa3              (cada etapa, su goroutine)
Productor–consumidor   productores ──► [cola acotada] ──► consumidores
Semáforo               limitar cuántas goroutines están EN una sección
Rate limiter           limitar cuántas operaciones POR unidad de tiempo
Pub/Sub                un publicador ──► TODOS los suscriptores (cada valor lo reciben todos)
Tee                    un flujo ──► dos copias
Bridge                 flujo de flujos ──► un flujo
Or-done                leer un channel con range y poder cancelar
```

### ¿Concurrencia o tasa?

| Quiero limitar... | Herramienta |
|--|--|
| cuántas tareas **a la vez** | worker pool o semáforo |
| cuántas operaciones **por segundo** | rate limiter |
| el tamaño de la cola entre etapas | channel con buffer (contrapresión) |

### Reglas comunes a todos los patrones

1. **Dueño del cierre.** Cada channel lo cierra su emisor, o un coordinador tras `wg.Wait()` (módulo 3).
2. **Toda goroutine tiene salida.** Cada envío y recepción relevante va en un `select` con `ctx.Done()` (módulo 5), o se verifica que otra parte cerrará el channel.
3. **Sin fugas.** Si el consumidor abandona antes, la cancelación debe liberar a las etapas anteriores. Cada ejercicio incluye un test de `NumGoroutine`.
4. **Acota todo:** workers, buffers, concurrencia. Lo que no se acota, se desborda en producción.
5. **Orden:** fan-out/fan-in y los pools **no preservan el orden** salvo que lo diseñes (índice + slice de resultados).
6. **Errores:** decide cómo viajan: en el resultado (`Resultado[T]`), en un channel aparte, o con `errgroup` (módulo 10).
7. **Genéricos** (`Merge[T]`, `Procesar[I,O]`) evitan duplicar cada patrón por tipo.
8. **Contrapresión:** una cola acotada hace que el productor rápido espere al lento. Una cola sin límite solo traslada el problema a la memoria.

### Elegir tamaño

| Carga | Workers |
|--|--|
| CPU-bound | ≈ `NumCPU` |
| I/O-bound | muy superior, limitado por el recurso externo (conexiones, API, memoria) |
| Mixta | medir (módulo 12) |

## Preguntas de repaso

1. ¿Cuál es la diferencia entre un worker pool y un semáforo?
2. ¿Por qué `Merge` no preserva el orden y cómo conservarías el orden de entrada en un pool?
3. ¿Qué ocurre con las etapas de un pipeline si el consumidor abandona sin cancelar?
4. ¿Qué es la contrapresión y qué pasa si usas una cola sin límite?
5. ¿Qué diferencia hay entre limitar concurrencia y limitar tasa?
6. En pub/sub, ¿por qué se descarta el mensaje para un suscriptor lento en lugar de bloquear?
7. ¿Para qué sirve el patrón `OrDone`? ¿Y `Tee`?
8. ¿Cómo evitas enviar a un channel ya cerrado en un broker con altas y bajas dinámicas?

## Lecturas

- [Go Concurrency Patterns: Pipelines and cancellation](https://go.dev/blog/pipelines)
- [Advanced Go Concurrency Patterns](https://go.dev/talks/2013/advconc.slide) — Sameer Ajmani
- *Concurrency in Go* — Katherine Cox-Buday (capítulo 4: *Concurrency Patterns in Go*)
- Documentación de [`golang.org/x/sync/semaphore`](https://pkg.go.dev/golang.org/x/sync/semaphore) y [`golang.org/x/time/rate`](https://pkg.go.dev/golang.org/x/time/rate)
