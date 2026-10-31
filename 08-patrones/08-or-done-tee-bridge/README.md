# Ejercicio 8: `OrDone`, `Tee` y `Bridge`

## Teoría

Tres utilidades pequeñas que aparecen en cualquier biblioteca de channels.

### `OrDone(ctx, in)`

Leer un channel con `range` es limpio, pero **no se puede cancelar**; hacerlo bien exige un `select` con `ctx.Done()` en cada lectura:

```go
for {
    select {
    case <-ctx.Done(): return
    case v, ok := <-in:
        if !ok { return }
        // usar v
    }
}
```

`OrDone` encapsula ese bucle y devuelve un canal que se cierra cuando `in` se cierra **o** `ctx` se cancela:

```go
for v := range OrDone(ctx, in) { ... }
```

Importante: la lectura *y* el reenvío van con `select` (si el consumidor abandona, no quedan goroutines).

### `Tee(ctx, in)`

Duplica un flujo en dos salidas. Cada valor se envía a **ambas**, en el orden en que cada consumidor esté listo. Detalle clave: se usan **dos variables locales** `a`, `b` que se ponen a `nil` al entregar, de modo que el segundo `select` solo espera a la que falta (técnica del módulo 4).

Consecuencia: `Tee` avanza al ritmo del consumidor **más lento**. Si uno de los dos no se lee, bloquea al otro hasta cancelar. Para desacoplar, añade buffers o descarta.

### `Bridge(ctx, flujo)`

Aplana un `<-chan <-chan T` en un `<-chan T`, consumiendo cada sub-channel por turnos. Casos: una etapa que produce un channel por lote, página o conexión. Mantiene el orden de los sub-channels.

## Qué hace el programa

```bash
go run .
go test -race -count=3 .
```

## Resultado de referencia

```
OrDone: 1 2 3
Tee: consumidor A = [10 20 30] | consumidor B (doble) = [20 40 60]
Bridge: 0 1 2 10 11 12 20 21 22
```

## Conclusiones

- `OrDone` convierte "leer con cancelación" en un `range`.
- `Tee` permite que dos consumidores independientes procesen el mismo flujo.
- `Bridge` preserva el orden de los sub-channels.

## Tests

`OrDone` respeta la cancelación aunque `in` no emita ni cierre; `Tee` entrega todo a ambos; `Bridge` mantiene orden incluso con sub-channels vacíos; sin fugas tras cancelar con un consumidor de `Tee` abandonado.

## Ejercicios propuestos

1. Generaliza `Tee` a `TeeN(ctx, in, n)`.
2. Añade buffers a `Tee` para que un consumidor lento no frene al otro hasta cierto punto.
3. Compón: `Bridge` de un flujo de páginas de una API paginada + `OrDone` + pipeline.
4. Implementa `OrChannel` (cierra cuando **cualquiera** de N channels `done` se cierra).
