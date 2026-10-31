# Ejercicio 6: panics con channels

## Teoría

Tres operaciones sobre channels provocan **panic** (más una al crearlos):

| Operación | Mensaje |
|--|--|
| enviar a un channel cerrado | `send on closed channel` |
| cerrar un channel ya cerrado | `close of closed channel` |
| cerrar un channel `nil` | `close of nil channel` |
| `make(chan T, n)` con `n < 0` | `makechan: size out of range` |

**Recibir** de un channel cerrado **no** es panic: devuelve el cero y `ok=false`.

Estos panics son síntoma de un diseño donde no está claro **quién cierra**. Prevención:

1. Un único dueño del cierre (el emisor o un coordinador).
2. Si hay varios emisores, ninguno cierra: un coordinador cierra tras `wg.Wait()`.
3. Si varios pueden querer cerrar, protege con `sync.Once`.
4. No uses `recover` para ocultarlos; arregla la propiedad del cierre.

## Qué hace el programa

Provoca cada panic dentro de una función con `defer recover()` solo para mostrar el mensaje.

```bash
go run .
```

## Resultado de referencia

```
enviar a cerrado         -> send on closed channel
cerrar dos veces         -> close of closed channel
cerrar nil               -> close of nil channel
recibir de cerrado       -> v=0 ok=false (sin panic)
buffer negativo          -> makechan: size out of range
```

## Conclusiones

- Quitar el `recover` hace que el panic termine el proceso entero (ver módulo 2, ejercicio 3).
- No existe forma de preguntar si un channel está cerrado de forma segura: el diseño debe saberlo.

## Ejercicios propuestos

1. Implementa un tipo `Cerrable` con `Close()` seguro usando `sync.Once`.
2. Escribe un escenario de 3 emisores y 1 receptor que provoque `send on closed channel` si el receptor cierra, y arréglalo.
3. Busca en el código de la librería estándar un `close(` y razona quién es el dueño.
