# Ejercicio 3: timeouts

## Teoría

Un **timeout** es "esperar como máximo `d`". En Go se escribe con `select`:

```go
select {
case v := <-op:
case <-time.After(d):
}
```

Tres variantes que no son equivalentes:

| Variante | Cuándo se reinicia | Implementación |
|--|--|--|
| Por operación | en cada operación | `time.After(d)` dentro del `select` |
| **Plazo global** | nunca | un `Timer` creado **antes** del bucle |
| **Inactividad** | tras cada mensaje | `t.Reset(d)` en cada mensaje |

**Trampa:** usar `time.After(d)` dentro de un `for`/`select` crea un timer nuevo por vuelta, lo que reinicia el plazo (no es un plazo global) y, antes de Go 1.23, acumulaba timers sin recolectar hasta expirar.

**El timeout no cancela el trabajo.** Solo abandonas la espera. La goroutine que hace el trabajo sigue hasta terminar. Por eso:

- El channel de resultados lleva **buffer 1** para que esa goroutine pueda enviar y salir aunque nadie escuche.
- Para cancelar de verdad se usa `context` (módulo 5).

`Timer.Stop()` libera el timer si ya no se necesita (`defer t.Stop()`).

## Qué hace el programa

1. Operación de 50 ms con límite de 200 ms (éxito) y de 500 ms con límite de 100 ms (timeout).
2. Productor que emite cada 60 ms con un **plazo global** de 250 ms.
3. Receptor que se rinde tras 120 ms **sin actividad**.

```bash
go run .
```

## Resultado de referencia

```
listo <nil>
 timeout tras 100ms

plazo agotado
mensajes recibidos antes del plazo: 4

mensaje 1 / 2 / 3
sin actividad durante 120ms: fin
```

## Conclusiones

- El plazo global cortó tras 4 mensajes (4×60 = 240 ms) aunque el productor seguía activo.
- La inactividad solo salta cuando pasan 120 ms sin mensajes, aunque en total lleve más tiempo.
- Elegir bien entre plazo y inactividad evita cortes injustos (descargas grandes) o esperas infinitas.

## Ejercicios propuestos

1. Cambia el plazo global por `time.After` dentro del bucle y observa que ahora **no** corta.
2. Sustituye el buffer 1 por un channel sin buffer y cuenta goroutines con `runtime.NumGoroutine()` tras el timeout.
3. Implementa `Reintentar(op, intentos, timeoutPorIntento)`.
