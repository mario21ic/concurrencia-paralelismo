# Ejercicio 5: semáforos

## Teoría

Un **semáforo** limita cuántas goroutines pueden estar simultáneamente en una sección. A diferencia del worker pool, **no** fija el número de goroutines: puedes lanzar muchas, pero solo N avanzan a la vez.

### Con un channel con buffer

```go
sem := make(chan struct{}, N)
sem <- struct{}{}        // adquirir (bloquea si hay N dentro)
defer func() { <-sem }() // liberar
```

Simple, idiomático y suficiente para casi todo. `struct{}` no ocupa memoria.

### `golang.org/x/sync/semaphore.Weighted`

Añade lo que el channel no da:

| Característica | Detalle |
|--|--|
| **Pesos** | `Acquire(ctx, n)` toma `n` unidades: tareas "pesadas" y "ligeras" comparten la misma capacidad |
| **Contexto** | `Acquire` se cancela con plazo/cancelación |
| **`TryAcquire(n)`** | no bloquea |
| **FIFO** | atiende en orden de llegada, así una tarea pesada no se muere de hambre |

Errores comunes:
- `Release` de más unidades de las adquiridas → pánico.
- `Acquire(n)` con `n` mayor que la capacidad → **nunca** se satisface (solo sale por el contexto).
- Olvidar `Release` (usa `defer`).

### Semáforo vs pool vs rate limiter

| Herramienta | Limita |
|--|--|
| Worker pool | goroutines creadas |
| Semáforo | goroutines **dentro** de una sección (p. ej. llamadas a una API) |
| Rate limiter | operaciones por unidad de tiempo |

## Qué hace el programa

1. 20 tareas de 30 ms con límite 4 (channel) y medición de la concurrencia máxima.
2. `Weighted(4)` con 8 tareas: 2 pesadas (peso 3) y 6 ligeras (peso 1).
3. `TryAcquire`, `Acquire` con plazo y petición imposible.

```bash
go run .
```

## Resultado de referencia

```
channel: 20 tareas de 30ms, límite 4 -> máx. simultáneas=4, tiempo=150ms (ideal 150ms)
weighted: capacidad 4, peso máximo en uso=4 (nunca >4), tiempo=130ms

TryAcquire(2): true
TryAcquire(1) con el semáforo lleno: false
Acquire con plazo de 50ms: context deadline exceeded | es DeadlineExceeded: true
Acquire(3) con capacidad 2: context deadline exceeded (imposible: solo sale por el plazo)
tras Release(2), TryAcquire(1): true
```

## Conclusiones

- Máximo observado = límite (4); 20 × 30 ms / 4 = 150 ms exactos.
- Con pesos, la capacidad total (4) jamás se supera aunque convivan tareas de pesos distintos.
- Un `Acquire` imposible solo termina por el plazo: es una fuente típica de bloqueos eternos.

## Ejercicios propuestos

1. Reescribe el semáforo de channel con soporte de contexto (`select` con `ctx.Done()` al adquirir).
2. Usa `Weighted` para limitar memoria: peso = tamaño del archivo en MB, capacidad = 100.
3. Crea 100 goroutines que descargan con límite 5 y mide el máximo con y sin límite.
