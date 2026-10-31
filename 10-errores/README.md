# Módulo 10. Manejo de errores y cancelación

**Objetivo:** gestionar fallos en programas concurrentes: propagarlos, agregarlos, evitar que un pánico mate el proceso, deduplicar trabajo repetido y apagar servicios de forma ordenada.

## Ejercicios

| # | Carpeta | Tema |
|---|---------|------|
| 1 | [01-errgroup](01-errgroup) | `errgroup`: `WithContext`, `SetLimit`, `TryGo` |
| 2 | [02-errors-join](02-errors-join) | `errors.Join`, `%w` múltiple, `Is`/`As` |
| 3 | [03-panics](03-panics) | `recover` en goroutines, `PanicError`, `GoSeguro` |
| 4 | [04-singleflight](04-singleflight) | `singleflight`: deduplicar, estampida de caché, trampa del contexto |
| 5 | [05-errores-en-pipelines](05-errores-en-pipelines) | Propagar errores en un pipeline con `errgroup` |
| 6 | [06-graceful-shutdown](06-graceful-shutdown) | Apagado ordenado de un servidor HTTP |
| 7 | [07-miniproyecto-crawler](07-miniproyecto-crawler) | **Mini-proyecto 3:** crawler concurrente |

```bash
cd 10-errores/01-errgroup && go run . && go test -race -timeout 60s .
```

> **Consejo:** ejecuta siempre los tests concurrentes con `-timeout`. Un test que se bloquea (por ejemplo, olvidar liberar un channel) cuelga la ejecución hasta los 10 minutos por defecto. Durante el desarrollo de este módulo me ocurrió con un test mal escrito.

## Resumen teórico

### Estrategias para errores concurrentes

| Estrategia | Herramienta | Cuándo |
|--|--|--|
| **Fallar rápido**: el primer error cancela todo | `errgroup.WithContext` | las tareas son partes de una misma operación (si una falla, el resultado no sirve) |
| **Recolectar todos** | slice `errs[i]` + `errors.Join` | tareas independientes y quieres un informe completo |
| **Error como dato** | `Resultado{Valor, Err}` | procesar lotes donde un fallo no invalida el resto |
| **Aislar pánicos** | `recover` + `PanicError` | una tarea defectuosa no debe tumbar el servicio |

### `errgroup` (`golang.org/x/sync/errgroup`)

```go
g, ctx := errgroup.WithContext(parent)   // ctx se cancela al primer error o al terminar Wait
g.SetLimit(n)                            // como mucho n goroutines; Go() bloquea si está lleno
g.Go(func() error { ... })               // TryGo no bloquea
err := g.Wait()                          // espera a todas; devuelve el PRIMER error
```

Reglas:
1. Las tareas deben usar **el `ctx` derivado**, no el externo; si no, nadie ve la cancelación.
2. `Wait` devuelve solo el **primer** error; los demás se pierden (usa `errors.Join` si los quieres todos).
3. `Wait` **siempre cancela** el contexto derivado al volver.
4. **No recupera pánicos** (verificado con `x/sync v0.24.0`): un pánico en una tarea mata el proceso.
5. `SetLimit` + `Go` desde **dentro** de una tarea del mismo grupo puede dar deadlock (las tareas ocupan todas las plazas esperando una libre).
6. Es **concurrencia estructurada**: nada sobrevive a `Wait`.

### Errores múltiples

`errors.Join(errs...)` devuelve un error que envuelve a todos (descarta los `nil`, `nil` si no queda ninguno). `errors.Is`/`As` recorren todo el árbol. `fmt.Errorf("... %w ... %w", a, b)` también (Go 1.20+).

### Pánicos y goroutines

- Un pánico no recuperado en **cualquier** goroutine termina el programa.
- `recover` solo funciona dentro de un `defer` de **la misma goroutine** que entra en pánico: el `recover` de `main` no atrapa el de otra goroutine.
- Recuperar todo ciegamente oculta bugs; se recupera en **fronteras** (cada petición, cada tarea de un pool) y se **registra con su traza**.
- `os.Exit` y `runtime.Goexit` no son pánicos.

### `singleflight`

Agrupa llamadas **simultáneas** con la misma clave en una sola ejecución; todas reciben el mismo resultado (y el mismo error). No es una caché. Su uso estrella: evitar la **estampida de caché** (*thundering herd*).

Trampa: la ejecución usa el `ctx` del **primer** llamador; si ese cancela, fallan todos. Solución: `DoChan` + `context.WithoutCancel` para la ejecución y `select` con el ctx de cada llamador.

### Apagado ordenado (*graceful shutdown*)

```
señal ──► ctx cancelado ──► dejar de aceptar ──► esperar en curso (hasta `gracia`) ──► forzar cierre
```

- Contexto raíz con `signal.NotifyContext`.
- Todos los componentes (servidor, workers) en un `errgroup` con el mismo contexto: si **uno** falla, **todos** paran.
- El plazo de gracia se crea con `context.WithoutCancel` (el contexto original ya está cancelado).
- `Shutdown(ctx)` espera a las peticiones en curso; si vence el plazo, `Close()` corta las conexiones.
- En Kubernetes: la gracia debe ser menor que `terminationGracePeriodSeconds`.

## Preguntas de repaso

1. ¿Qué ocurre con las demás tareas cuando una falla en un `errgroup` **sin** `WithContext`? ¿Y con él?
2. ¿Por qué es un bug usar el contexto externo dentro de las tareas de un `errgroup`?
3. ¿Qué errores devuelve `Wait` si fallan tres tareas? ¿Cómo conservarías todos?
4. ¿Por qué el `recover` de `main` no atrapa el pánico de otra goroutine?
5. ¿En qué se diferencia `singleflight` de una caché?
6. ¿Qué pasa si el primer llamador de `singleflight.Do` cancela su contexto?
7. ¿Por qué el plazo de gracia usa `context.WithoutCancel`?
8. ¿Qué diferencia hay entre `Shutdown` y `Close`?
9. En un pipeline con `errgroup`, ¿quién cierra los channels y por qué hace falta un sub-grupo?

## Lecturas

- Documentación de [`errgroup`](https://pkg.go.dev/golang.org/x/sync/errgroup) y [`singleflight`](https://pkg.go.dev/golang.org/x/sync/singleflight)
- [Working with Errors in Go 1.13](https://go.dev/blog/go1.13-errors) y notas de Go 1.20 (`Join`, `%w` múltiple)
- [`http.Server.Shutdown`](https://pkg.go.dev/net/http#Server.Shutdown)
- [Defer, Panic, and Recover](https://go.dev/blog/defer-panic-and-recover)
