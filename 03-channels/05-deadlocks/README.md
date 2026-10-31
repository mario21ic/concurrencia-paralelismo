# Ejercicio 5: deadlocks

## Teoría

Un **deadlock** ocurre cuando un conjunto de goroutines espera eternamente algo que solo otra del conjunto podría dar.

El runtime detecta el caso extremo: si **todas** las goroutines están bloqueadas, aborta con

```
fatal error: all goroutines are asleep - deadlock!
```

e imprime el estado de cada goroutine (`chan send`, `chan receive`, `sync.WaitGroup.Wait`, `sync.Mutex.Lock`…), muy útil para diagnosticar.

**Lo que NO detecta:** si queda alguna goroutine viva (un servidor HTTP, un `time.Sleep`, un `select` con timer…), el programa simplemente se cuelga o filtra goroutines en silencio. En servidores reales casi nunca verás el mensaje.

### Causas habituales

| Modo | Causa | Arreglo |
|--|--|--|
| `envio-sin-receptor` | envío a channel sin buffer sin nadie leyendo (en la misma goroutine) | lanzar el receptor primero/en otra goroutine |
| `recepcion-sin-emisor` | recibir cuando nadie enviará | asegurar un emisor o usar `select` con salida |
| `range-sin-close` | el emisor no cierra | `defer close(ch)` en el emisor |
| `waitgroup` | `Add` sin su `Done` | usar `wg.Go`, o `defer wg.Done()` |
| `mutex` | `Lock` dos veces (no es reentrante) | refactorizar para no re-entrar |
| `parcial` | goroutines bloqueadas mientras `main` sigue vivo | fuga: dar siempre una salida (`done`/`context`) |

Otros deadlocks clásicos: orden inconsistente de locks (A→B vs B→A), dos goroutines enviándose mutuamente en channels sin buffer, y bloquear en un callback mientras se sostiene un lock.

## Qué hace el programa

```bash
go run . -modo=envio-sin-receptor
go run . -modo=recepcion-sin-emisor
go run . -modo=range-sin-close
go run . -modo=waitgroup
go run . -modo=mutex
go run . -modo=parcial
```

## Resultado de referencia

```
-modo=envio-sin-receptor
fatal error: all goroutines are asleep - deadlock!
goroutine 1 [chan send]:

-modo=range-sin-close
recibido 0 / 1 / 2
fatal error: all goroutines are asleep - deadlock!

-modo=waitgroup   -> goroutine 1 [sync.WaitGroup.Wait]
-modo=mutex       -> goroutine 1 [sync.Mutex.Lock]

-modo=parcial
goroutines vivas (fugadas): 5      <- NO hay error; es una fuga
```

## Conclusiones

- Lee el estado entre corchetes: dice exactamente en qué operación está atascada cada goroutine.
- Añadir buffer **oculta** el deadlock hasta que se llene; arregla el diseño, no el síntoma.
- Las fugas (`parcial`) son más peligrosas que los deadlocks totales porque no fallan: ver módulo 11.

## Ejercicios propuestos

1. Arregla cada modo con el cambio mínimo.
2. Construye un deadlock con dos mutex y dos goroutines (orden A→B / B→A) y ejecútalo.
3. Ejecuta `parcial` con `net/http/pprof` activo y observa el perfil `goroutine`.
