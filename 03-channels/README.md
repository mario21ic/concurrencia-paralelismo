# Módulo 3. Channels

**Objetivo:** comunicar goroutines de forma segura y reconocer (y evitar) los errores clásicos: deadlocks, panics y fugas.

## Ejercicios

| # | Carpeta | Tema |
|---|---------|------|
| 1 | [01-sin-buffer-vs-buffer](01-sin-buffer-vs-buffer) | Semántica de sincronización |
| 2 | [02-direccionalidad](02-direccionalidad) | `chan<-`, `<-chan` |
| 3 | [03-close-range](03-close-range) | `close`, `range`, `v, ok := <-ch` |
| 4 | [04-nil-channels](04-nil-channels) | Channels `nil` y `select` |
| 5 | [05-deadlocks](05-deadlocks) | Provocar y entender deadlocks |
| 6 | [06-panics](06-panics) | Provocar y entender panics |
| 7 | [07-ping-pong](07-ping-pong) | Ejercicio: ping-pong y costo de sincronizar |
| 8 | [08-generador](08-generador) | Ejercicio: generador y fugas |
| 9 | [09-miniproyecto-contador-palabras](09-miniproyecto-contador-palabras) | **Mini-proyecto 1** |

```bash
cd 03-channels/01-sin-buffer-vs-buffer && go run .
```

## Resumen teórico

Un channel es un conducto **tipado y sincronizado** entre goroutines: `ch := make(chan T, cap)`.

| Operación | Sin buffer | Con buffer | `nil` | Cerrado |
|--|--|--|--|--|
| `ch <- v` | espera receptor | espera si está lleno | bloquea siempre | **panic** |
| `<-ch` | espera emisor | espera si está vacío | bloquea siempre | devuelve cero, `ok=false` (tras drenar) |
| `close(ch)` | ok | ok | **panic** | **panic** |
| `len`/`cap` | 0 / 0 | ocupados / capacidad | 0 / 0 | igual |

Reglas de oro:

1. **Quien envía cierra**, nunca el receptor. Con varios emisores, uno coordinador cierra tras un `WaitGroup`.
2. **Cerrar es opcional**: solo es necesario si alguien espera el fin (`range`) o como señal de difusión.
3. Un channel sin buffer **sincroniza**; uno con buffer **desacopla** hasta llenarse. No uses buffer para "arreglar" un deadlock: lo oculta.
4. Usa tipos direccionales en firmas de funciones: documentan intención y el compilador lo verifica.
5. `chan struct{}` para señales sin datos (0 bytes por elemento).
6. Toda goroutine que envía o recibe debe tener una **salida** (cierre, `done`, `context`) o terminará filtrada.
7. El detector de deadlocks solo salta cuando **todas** las goroutines están bloqueadas; las fugas parciales pasan desapercibidas.

### Happens-before

- El envío ocurre antes de que la recepción correspondiente complete.
- `close` ocurre antes de una recepción que retorna por el cierre.
- En sin buffer, la recepción ocurre antes de que el envío complete.
- En buffer de capacidad C, la recepción k-ésima ocurre antes de que el envío (k+C)-ésimo complete.

## Preguntas de repaso

1. ¿Qué diferencia hay entre `make(chan int)` y `make(chan int, 1)` en cuanto a sincronización?
2. ¿Qué ocurre al recibir de un channel cerrado con valores pendientes en el buffer?
3. ¿Por qué un `select` con un channel que se cierra puede girar sin parar, y cómo se evita?
4. ¿Por qué el receptor no debe cerrar el channel?
5. ¿Qué ve el detector de deadlocks y qué no?

## Lecturas

- [Go Concurrency Patterns (Pike)](https://go.dev/talks/2012/concurrency.slide)
- [Share Memory By Communicating](https://go.dev/blog/codelab-share)
- [The Go Memory Model](https://go.dev/ref/mem)
- *Understanding Channels* — Kavya Joshi (GopherCon 2017)
