# Concepto 1 · Lanzar tareas y esperar a que terminen

> **Pregunta guía:** Tengo varias cosas que hacer a la vez (descargas, consultas, validaciones). ¿Cómo las lanzo y cómo sé cuándo acabaron, y qué hago si una falla?

Es el primer problema real de la concurrencia: **solapar esperas**. No hace falta tener más núcleos: mientras una goroutine espera la red, otras avanzan. Es *concurrencia* (estructura), todavía no *paralelismo* (aprovechar núcleos).

## Escalera de casos

| Nivel | Caso | Idea | Feature principal |
|--|--|--|--|
| 1 · básico | [1-basico-waitgroup](./1-basico-waitgroup) | 8 «descargas» de 100 ms: 800 ms en serie, 100 ms a la vez | `go`, `sync.WaitGroup.Go` |
| 2 · intermedio | [2-intermedio-errgroup](./2-intermedio-errgroup) | validar un lote; el primer error cancela el resto; máx. N a la vez | `errgroup`, `SetLimit`, `context` |
| 3 · avanzado | [3-avanzado-degradacion](./3-avanzado-degradacion) | panel con 3 servicios: dos esenciales, uno opcional, plazo global | `errgroup` + `WithTimeout` + decisión de diseño |

## Cómo elegir

| Si necesitas… | Usa |
|--|--|
| lanzar N tareas y esperar a todas, sin errores que propagar | `WaitGroup.Go` |
| lo anterior pero **parar al primer error** y/o **limitar** la concurrencia | `errgroup` |
| que algunas partes sean **opcionales** (degradar en vez de fallar) | `errgroup` + tú decides qué errores devuelves |
| esperar a UNA sola cosa que ocurre en otra goroutine | un `channel` que se cierra (`<-hecho`) |
