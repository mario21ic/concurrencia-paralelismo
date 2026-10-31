# Ejercicio 4: `Cause`, `AfterFunc` y `WithoutCancel`

## Teoría

### `WithCancelCause` y `Cause`

`ctx.Err()` solo distingue "cancelado" de "plazo vencido". Con `WithCancelCause` se cancela con un error que explica **por qué**:

```go
ctx, cancel := context.WithCancelCause(parent)
cancel(ErrCuotaExcedida)
ctx.Err()            // context.Canceled
context.Cause(ctx)   // ErrCuotaExcedida
```

- `context.Cause(ctx)` devuelve la causa; si no la hay, devuelve `ctx.Err()`.
- `WithTimeoutCause(parent, d, cause)` y `WithDeadlineCause` asignan la causa para el vencimiento por plazo.
- Muy útil en logs y métricas: "se canceló porque el cliente se desconectó" frente a "porque venció el límite de la base de datos".

### `AfterFunc`

`stop := context.AfterFunc(ctx, f)` ejecuta `f` en **su propia goroutine** cuando `ctx` termina. Sirve para limpiar recursos o abortar operaciones que no entienden de contextos (p. ej. `conn.SetDeadline(time.Now())` para desbloquear una lectura).

- `stop()` desregistra: devuelve `true` si evitó que `f` corriera, `false` si ya empezó o se ejecutó.
- `f` no se espera: si necesitas saber cuándo termina, sincronízalo tú.

### `WithoutCancel`

Devuelve un contexto que conserva los **valores** del padre pero **no hereda su cancelación ni su plazo** (`Done()` es `nil`). Casos: auditoría o limpieza que debe completarse aunque la petición original se haya cancelado.

Cuidado: crea trabajo que ya no está atado al ciclo de vida de la petición. Acótalo con su propio `WithTimeout`.

## Qué hace el programa

1. Cancela con causa y muestra `Err` frente a `Cause`.
2. `WithTimeoutCause` y su causa tras vencer.
3. `AfterFunc` que corre al cancelar; `stop()` antes/después.
4. `WithoutCancel` conservando el usuario tras cancelar la petición.

```bash
go run .
```

## Resultado de referencia

```
Err:    context canceled
Cause:  cuota excedida | es errCuotaExcedida: true
Err:    context deadline exceeded
Cause:  la base de datos no respondió
AfterFunc: liberando recursos...
stop() tras ejecutarse devuelve: false (false = ya corrió)
stop() antes de cancelar devuelve: true (true = se evitó)
petición cancelada: context canceled
auditoría: Err = <nil> | usuario = ana
```

## Conclusiones

- `Err()` sigue siendo `Canceled`/`DeadlineExceeded`; la información rica va en `Cause`.
- `AfterFunc` + `stop` es la forma limpia de enganchar limpieza a un contexto.
- `WithoutCancel` desacopla la cancelación pero mantiene metadatos.

## Ejercicios propuestos

1. Cancela con `fmt.Errorf("usuario %s: %w", ...)` y comprueba `errors.Is`.
2. Usa `AfterFunc` para desbloquear un `net.Conn.Read` bloqueado con `SetReadDeadline(time.Now())`.
3. Dale a la auditoría su propio `WithTimeout(WithoutCancel(ctx), 2*time.Second)`.
