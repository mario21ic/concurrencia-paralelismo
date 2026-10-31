# Ejercicio 2: plazos y HTTP con contexto

## Teoría

- `WithTimeout(parent, d)` = `WithDeadline(parent, time.Now().Add(d))`. Al vencer, `Done()` se cierra y `Err()` es `context.DeadlineExceeded`.
- **Diferencia de errores:**
  - `context.Canceled`: alguien llamó a `cancel()`.
  - `context.DeadlineExceeded`: venció el plazo (implementa `Timeout() bool` → `true`).
  - Compáralos con `errors.Is`, no con `==` (llegan envueltos).
- Un **hijo no puede alargar** el plazo del padre: si pide 10 s bajo un padre de 100 ms, su `Deadline()` es el del padre. Siempre manda el plazo más corto.
- `ctx.Deadline()` permite a una función decidir si le da tiempo (p. ej. no iniciar una operación de 2 s si quedan 500 ms).
- `defer cancel()` sigue siendo obligatorio aunque el plazo venza solo.
- **En red, el contexto cruza la conexión:** `http.NewRequestWithContext(ctx, ...)` aborta la petición del cliente al cancelar. En el servidor, `r.Context()` se cancela cuando el cliente se desconecta, así el handler puede dejar de trabajar para nadie.

Comparación con el módulo 4: `time.After` abandona la **espera**; `context` además **propaga** la orden a quien hace el trabajo.

## Qué hace el programa

1. `WithTimeout` de 100 ms sobre una operación de 300 ms.
2. `WithDeadline` con instante absoluto.
3. Hijo con 10 s bajo un padre de 100 ms: hereda el del padre.
4. Distinción `Canceled` / `DeadlineExceeded`.
5. Cliente HTTP con timeout de 100 ms contra un servidor que tarda 500 ms; el servidor detecta la cancelación.

```bash
go run .
```

## Resultado de referencia

```
timeout: context deadline exceeded | es DeadlineExceeded: true
deadline en 50ms (ok=true)
hijo hereda el plazo más corto del padre: true
cancelado manualmente: context canceled | es Canceled: true
cliente: Get "http://127.0.0.1:54698": context deadline exceeded (tras 100ms)
servidor: el cliente canceló (context canceled)
```

## Conclusiones

- El cliente falló a los 100 ms exactos y el servidor **se enteró** y paró; con `time.After` no habría sabido nada.
- El error del cliente es `DeadlineExceeded`; el del servidor, `Canceled` (el contexto del servidor se cancela por desconexión, no por plazo).

## Ejercicios propuestos

1. Haz que `operacion` compruebe `ctx.Deadline()` y falle rápido si no queda tiempo suficiente.
2. Añade reintentos: 3 intentos con un `WithTimeout` por intento dentro de un plazo global.
3. Usa `exec.CommandContext(ctx, "sleep", "5")` con timeout de 1 s y observa cómo mata el proceso.
