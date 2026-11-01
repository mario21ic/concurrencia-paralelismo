# Concepto 5 · Limitar recursos y proteger sistemas

> **Pregunta guía:** Tengo más trabajo del que el sistema (o el servicio remoto) aguanta. ¿Cómo lo acoto, cuándo digo «no», y cómo evito que un cliente abuse?

La concurrencia sin límites es una forma de ataque a uno mismo. Hay **tres** cosas distintas que se pueden limitar, y no son intercambiables: la **concurrencia** (cuántas a la vez), la **cola** (cuántas esperan) y la **tasa** (cuántas por segundo).

## Escalera de casos

| Nivel | Caso | Idea | Feature principal |
|--|--|--|--|
| 1 · básico | [1-basico-semaforo](./1-basico-semaforo) | no más de N operaciones simultáneas | `chan struct{}`, `errgroup.SetLimit`, `semaphore.Weighted` |
| 2 · intermedio | [2-intermedio-worker-pool](./2-intermedio-worker-pool) | workers fijos + cola acotada; bloquear o rechazar | `chan func()`, `select` con `default` |
| 3 · avanzado | [3-avanzado-rate-limit](./3-avanzado-rate-limit) | token bucket por cliente y `429 Retry-After` | `golang.org/x/time/rate`, `http.Handler` |

## Cómo elegir

| Quieres limitar… | Mecanismo | Si se supera… |
|--|--|--|
| **cuántas a la vez** | semáforo / pool de N workers | las demás esperan (o `errgroup.SetLimit` bloquea el `Go`) |
| **cuántas esperan** | cola acotada (`chan` con buffer) | bloquear al productor (contrapresión) o **rechazar** (503) |
| **cuántas por segundo** | rate limiter (token bucket) | rechazar con 429 y `Retry-After`, o esperar (`Wait`) |
| **memoria/peso total** | `semaphore.Weighted` | esperar a que se libere presupuesto |
| **tiempo de cada una** | `context.WithTimeout` | cancelar |
