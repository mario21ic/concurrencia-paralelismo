# Ejercicio 3: `close`, `range` y `v, ok := <-ch`

## Teoría

`close(ch)` indica "no se enviarán más valores". No libera memoria (eso lo hace el GC) ni es obligatorio.

Tras cerrar:
- Los valores ya en el buffer **se pueden seguir recibiendo**.
- Cuando se agota, cada recepción devuelve inmediatamente el **valor cero** del tipo y `ok == false`.
- Enviar o volver a cerrar produce **panic**.

Formas de recibir:

```go
v := <-ch            // no distingue "cero real" de "cerrado"
v, ok := <-ch        // ok=false si cerrado y vacío
for v := range ch {} // termina al cerrarse y vaciarse
```

**`close` como difusión:** un envío despierta a *un* receptor; un `close` despierta a *todos*. Es el mecanismo base de las señales de cancelación (`done`, `ctx.Done()`).

**Quién cierra:** el único emisor, o un coordinador que espera a todos los emisores (`wg.Wait(); close(ch)`).

## Qué hace el programa

1. Cierra un buffer con 2 valores y recibe 4 veces.
2. Usa `range` sobre un channel que cierra el productor.
3. Lanza 3 workers que esperan `<-listo` y los libera con un solo `close`.

```bash
go run .
```

## Resultado de referencia

```
v=10 ok=true
v=20 ok=true            <- el buffer se drena aunque esté cerrado
v=0 ok=false
v=0 ok=false
range: a / b / c
soltando a todos...
worker 0 arrancó / 1 / 2
```

## Conclusiones

- `ok` es la forma de distinguir un cero real de un channel cerrado.
- `range` sin `close` del emisor se queda esperando para siempre (ver ejercicio 5).
- Un `close` libera a todos los receptores bloqueados.

## Ejercicios propuestos

1. Cambia `chan struct{}` por `chan bool` y envía `true` tres veces: ¿cuántos workers se liberan? ¿por qué close es mejor?
2. Implementa "N emisores y 1 receptor" cerrando el channel con un `WaitGroup` coordinador.
3. Qué ocurre si dos goroutines intentan `close(ch)`? Usa `sync.Once` para evitarlo.
