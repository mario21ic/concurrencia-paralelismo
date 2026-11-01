# Caso 3.1 — Productor y consumidor  ·  `básico`

> **Problema:** Una goroutine genera valores y otra los procesa a su ritmo. En lugar de compartir una variable, el dato **se entrega** por un channel.

## Qué features usa y por qué

| Feature de Go | Papel en este caso |
|--|--|
| `chan int` | transfiere el valor y sincroniza a las dos goroutines |
| `close(ch)` | el productor avisa «no hay más» |
| `for v := range ch` | el consumidor termina solo cuando el channel se cierra |
| `make(chan T, n)` | el buffer es un amortiguador entre ritmos distintos |
| `<-chan T` / `chan<- T` | la firma documenta quién envía y quién recibe |

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ Hay un flujo claro de datos de A a B.
- ✅ Quieres que el ritmo lo marque el consumidor (sin buffer) o absorber ráfagas (con buffer).
- ❌ Solo necesitas proteger un contador → mutex/atomic (concepto 2).
- ❌ Recorrido secuencial sin concurrencia → iterador (módulo 9).
- ❌ Buffer enorme «para que no se bloquee»: esconde el problema y gasta memoria.

## Cómo ejecutarlo

```bash
go run .
go test -race -v .
```

## Qué observar (salida real, Apple M1, Go 1.26.5)

```
suma de cuadrados 1..10 = 385
buffer=0 → a los 25 ms el productor ya envió 1 de 10
buffer=1 → a los 25 ms el productor ya envió 2 de 10
buffer=4 → a los 25 ms el productor ya envió 5 de 10
```

## Errores típicos

- **Olvidar `close`**: el `range` del consumidor se queda esperando para siempre (interbloqueo).
- **Cerrar desde el receptor**, o cerrar dos veces → pánico. Regla: quien envía, cierra.
- Enviar a un channel cerrado → pánico.
- Un channel sin buffer y un solo goroutine que envía y recibe: se bloquea a sí misma.

## Profundiza en el curso

- [sin buffer frente a con buffer](../../../03-channels/01-sin-buffer-vs-buffer)
- [`close` y `range`](../../../03-channels/03-close-range)
- [los interbloqueos típicos](../../../03-channels/05-deadlocks)
- [`<-chan` y `chan<-`](../../../03-channels/02-direccionalidad)
- [el patrón completo](../../../08-patrones/04-productor-consumidor)

## Prueba tú

1. Quita el `close` y observa el `fatal error: all goroutines are asleep`.
2. Haz dos productores y un consumidor: ¿quién cierra?
3. Cambia el consumidor por un `select` con un plazo.
