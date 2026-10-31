# Módulo 5. `context`

**Objetivo:** propagar cancelación, plazos y metadatos a través de goroutines y capas de código, y apagar programas de forma ordenada.

## Ejercicios

| # | Carpeta | Tema |
|---|---------|------|
| 1 | [01-withcancel](01-withcancel) | `WithCancel` y el árbol de contextos |
| 2 | [02-timeout-deadline](02-timeout-deadline) | `WithTimeout`, `WithDeadline`, HTTP con contexto |
| 3 | [03-cancelacion-cooperativa](03-cancelacion-cooperativa) | Revisar `ctx.Done()` en bucles largos |
| 4 | [04-cause-afterfunc-withoutcancel](04-cause-afterfunc-withoutcancel) | `WithCancelCause`, `Cause`, `AfterFunc`, `WithoutCancel` |
| 5 | [05-withvalue](05-withvalue) | `WithValue`: cuándo sí y cuándo no |
| 6 | [06-servidor-rapido-con-ctx](06-servidor-rapido-con-ctx) | Módulo 4, ejercicio 7, con cancelación real |
| 7 | [07-worker-ctrl-c](07-worker-ctrl-c) | **Ejercicio:** worker que se detiene con `Ctrl+C` |

```bash
cd 05-context/01-withcancel && go run .
```

## Resumen teórico

`context.Context` es una interfaz inmutable con cuatro métodos:

```go
type Context interface {
    Deadline() (deadline time.Time, ok bool)
    Done() <-chan struct{}   // se cierra al cancelar o vencer el plazo
    Err() error              // nil | context.Canceled | context.DeadlineExceeded
    Value(key any) any
}
```

### Constructores

| Función | Para qué |
|--|--|
| `context.Background()` | raíz: `main`, tests, inicio de una petición |
| `context.TODO()` | marcador "aún no sé qué contexto poner" |
| `WithCancel(parent)` | cancelación manual (`cancel()`) |
| `WithTimeout(parent, d)` / `WithDeadline(parent, t)` | plazo relativo / absoluto |
| `WithCancelCause`, `WithTimeoutCause` | cancelar con una **causa** (`context.Cause(ctx)`) |
| `WithValue(parent, k, v)` | metadatos de la petición |
| `WithoutCancel(parent)` | conserva valores, ignora la cancelación del padre |
| `AfterFunc(ctx, f)` | ejecuta `f` cuando ctx termina |
| `signal.NotifyContext` | contexto cancelado por señales del SO |

### Reglas (del propio paquete)

1. **`ctx` es el primer parámetro**, nombrado `ctx`: `func Hacer(ctx context.Context, ...)`.
2. **No guardes contextos en structs**; pásalos por parámetro (excepción: tipos que representan una única petición).
3. **Nunca pases `nil`**; si dudas, `context.TODO()`.
4. **Siempre llama a `cancel`** (`defer cancel()`) tras `WithCancel`/`WithTimeout`/`WithDeadline`, aunque el plazo venza solo: libera timers y la relación padre→hijo. `go vet` avisa (`lostcancel`).
5. **Quien crea el contexto con cancel es quien lo cancela.**
6. `WithValue` solo para datos *de ámbito de petición* que cruzan APIs (ID de traza, usuario autenticado), **no** para parámetros opcionales de funciones. Clave: tipo propio no exportado.

### La cancelación es cooperativa

`cancel()` solo cierra un channel. Nada para a una goroutine por la fuerza: **debe mirar** `ctx.Done()` o `ctx.Err()`:

- Bloqueos: `select` con `<-ctx.Done()` (no `time.Sleep`).
- Bucles de CPU: `ctx.Err()` cada N iteraciones.
- Llamadas de librería que aceptan `ctx` (`http.NewRequestWithContext`, `db.QueryContext`, `exec.CommandContext`): pásalo siempre.

### Propagación

Es un **árbol**: cancelar un nodo cancela todo su subárbol, nunca hacia arriba. Un hijo puede acortar el plazo del padre, no alargarlo.

## Preguntas de repaso

1. ¿Qué devuelven `ctx.Err()` y `context.Cause(ctx)` cuando vence un `WithTimeout`?
2. ¿Por qué hay que llamar a `cancel` aunque el contexto expire solo?
3. ¿Cancelar un contexto detiene la goroutine? ¿Qué tiene que hacer ella?
4. ¿Por qué la clave de `WithValue` debe ser un tipo propio?
5. ¿Qué diferencia hay entre `WithoutCancel` y pasar `context.Background()`?
6. ¿Qué ocurre con la segunda señal tras `stop()` de `NotifyContext`?

## Lecturas

- [Go Concurrency Patterns: Context](https://go.dev/blog/context)
- [Contexts and structs](https://go.dev/blog/context-and-structs)
- Documentación de [`context`](https://pkg.go.dev/context) y [`os/signal`](https://pkg.go.dev/os/signal#NotifyContext)
