# Ejercicio 1: sin buffer vs con buffer

## Teoría

**Sin buffer** (`make(chan T)`): el envío y la recepción deben **encontrarse** (*rendezvous*). El que llega primero espera al otro. Es un mecanismo de sincronización: cuando `ch <- v` retorna, sabes que alguien ya recibió `v`.

**Con buffer** (`make(chan T, n)`): una cola FIFO de capacidad `n`.
- El envío solo bloquea si el buffer está lleno.
- La recepción solo bloquea si está vacío.
- `len(ch)` = elementos en cola; `cap(ch)` = capacidad.
- Desacopla emisor y receptor, absorbe ráfagas, pero **no** garantiza que alguien haya recibido el dato.

Cuándo usar cada uno:

| Sin buffer | Con buffer |
|--|--|
| quieres sincronizar / confirmar entrega | absorber picos, limitar concurrencia (semáforo) |
| diseño por defecto | resultados cuyo número conoces (evita goroutines bloqueadas) |
| depurar deadlocks más fácil | no como parche para un deadlock |

## Qué hace el programa

1. Sin buffer: el receptor tarda 200 ms en llegar; el envío no se completa hasta entonces.
2. Con buffer (cap 3): usa `select` con `default` para enviar sin bloquear hasta llenarlo, y luego lo vacía (FIFO).

```bash
go run .
```

## Resultado de referencia

```
[200ms] receptor listo
recibido: hola
[200ms] envío completado          <- el envío esperó 200 ms

enviado 1  -> len=1 cap=3
enviado 2  -> len=2 cap=3
enviado 3  -> len=3 cap=3
enviar 4 bloquearía: buffer lleno
recibido 1 / 2 / 3                <- FIFO
```

## Conclusiones

- Sin buffer: el envío termina cuando el receptor recibe.
- Con buffer: los 3 primeros envíos son inmediatos; el cuarto bloquearía.
- `select` con `default` es la forma de probar un envío/recepción sin bloquear.

## Ejercicios propuestos

1. Cambia el buffer a 0 y ejecuta la segunda parte: ¿qué ocurre en el primer envío?
2. Mide el tiempo de enviar 1 000 000 de ints con buffer 0, 1, 100 y 10 000 a un consumidor.
3. Usa un `chan struct{}` con buffer 3 como semáforo para limitar a 3 goroutines simultáneas.
