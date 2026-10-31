# Ejercicio 3: la cancelación es cooperativa

## Teoría

El runtime **no puede** matar una goroutine. `cancel()` únicamente cierra un channel; si el código no lo consulta, sigue como si nada.

| Situación | Cómo reaccionar a la cancelación |
|--|--|
| Esperar un evento | `select { case <-evento: ... case <-ctx.Done(): ... }` |
| Pausar | **no** `time.Sleep`; usa un `Timer` en `select` con `ctx.Done()` |
| Bucle de CPU | `if ctx.Err() != nil { return }` cada N iteraciones |
| Llamadas bloqueantes de librerías | usar sus variantes con `ctx` |
| Llamada que no admite `ctx` | ejecutarla en otra goroutine y esperar con `select` (la goroutine seguirá hasta que termine) |

### ¿Cada cuánto revisar?

`ctx.Err()` toma un lock interno; hacerlo en cada iteración de un bucle muy caliente cuesta. Revisar cada 1 000–10 000 iteraciones suele bastar. Compromiso: latencia de cancelación vs sobrecoste.

### Qué devolver

Al detectar la cancelación, devuelve `ctx.Err()` (o `context.Cause(ctx)`) envuelto con contexto, para que el llamador pueda usar `errors.Is(err, context.Canceled)`. Limpia lo que corresponda con `defer`.

## Qué hace el programa

Lanza tres variantes en una goroutine, espera 100 ms, cancela y mide cuánto tarda en parar:

1. `ignora`: usa `time.Sleep(500ms)` y no mira el contexto.
2. `bucleBloqueante`: espera con `select` + timer.
3. `bucleCPU`: cálculo continuo con `ctx.Err()` cada 10 000 iteraciones.

```bash
go run .
```

## Resultado de referencia

```
ignora el contexto:          tardó en parar: 400ms
select con ctx.Done():       tardó en parar: 0s
bucle CPU con ctx.Err():     tardó en parar: 0s
```

## Conclusiones

- La primera sigue los 400 ms restantes de su `Sleep` pese a estar cancelada.
- Las otras paran de inmediato.
- Una goroutine que no coopera es un **leak** esperando ocurrir.

## Ejercicios propuestos

1. Cambia el intervalo de `bucleCPU` a 1 y a 100 000 000 y mide latencia de cancelación y número de iteraciones.
2. Escribe una función que envuelva una llamada que **no** acepta contexto para que sí lo respete (goroutine + `select`). ¿Qué queda corriendo?
3. Implementa `Dormir(ctx, d) error` reutilizable.
