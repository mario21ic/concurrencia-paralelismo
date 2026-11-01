# Concepto 4 · Tiempo y cancelación

> **Pregunta guía:** ¿Cómo evito esperar para siempre, cómo reintento sin empeorar las cosas y cómo paro un programa sin perder trabajo?

Todo lo que espera (red, disco, otra goroutine) puede **no terminar nunca**. En Go el tiempo y la cancelación se expresan con `time` + `select` y, sobre todo, con **`context.Context`**: un valor que viaja por la cadena de llamadas diciendo «se acabó el plazo» o «cancela esto».

## Escalera de casos

| Nivel | Caso | Idea | Feature principal |
|--|--|--|--|
| 1 · básico | [1-basico-timeout](./1-basico-timeout) | poner un plazo a una operación lenta (3 formas, 2 malas) | `context.WithTimeout`, `select`, `time.Timer` |
| 2 · intermedio | [2-intermedio-reintentos](./2-intermedio-reintentos) | reintentar con backoff exponencial + jitter, sin reintentar lo permanente | `time.Timer`, `errors.Is`, `ctx` |
| 3 · avanzado | [3-avanzado-apagado-ordenado](./3-avanzado-apagado-ordenado) | cerrar un servicio: drenar, gracia y cancelar lo que no acabe | `close`, `RWMutex`, `WaitGroup`, `context` |

## Cómo elegir

| Quieres… | Usa |
|--|--|
| un plazo para UNA operación | `context.WithTimeout(ctx, d)` y pasar `ctx` a la operación |
| que **todo** un árbol de operaciones pare junto | un `ctx` padre; `cancel()` lo para todo |
| esperar «lo que ocurra primero» (resultado, plazo, cancelación) | `select` |
| algo cada X tiempo | `time.Ticker` (recuerda `Stop`) |
| ejecutar algo en X tiempo y poder anularlo | `time.AfterFunc` / `time.Timer` |
| saber **por qué** se canceló | `context.WithCancelCause` / `context.Cause` |
| terminar el trabajo en curso al apagar | `signal.NotifyContext` + gracia + cancelar lo restante |
