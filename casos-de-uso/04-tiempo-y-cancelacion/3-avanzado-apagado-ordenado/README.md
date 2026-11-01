# Caso 4.3 — Apagar un servicio sin perder trabajo  ·  `avanzado`

> **Problema:** Al recibir SIGTERM hay que: dejar de aceptar tareas, terminar las ya aceptadas, no esperar más de una *gracia*, y no provocar `send on closed channel` si alguien envía justo mientras se cierra.

## Qué features usa y por qué

| Feature de Go | Papel en este caso |
|--|--|
| `close(cola)` | avisa a los workers; `range` drena lo pendiente y termina |
| `sync.RWMutex` | `Enviar` toma `RLock`; `Cerrar` toma `Lock` antes de cerrar el channel: nunca se envía a uno cerrado |
| `sync.WaitGroup.Go` | esperar a todos los workers |
| `context.WithCancel` + temporizador | pasada la gracia, se cancela lo que siga en curso |
| `goleak` | al volver de `Cerrar` no queda ninguna goroutine |

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ Cualquier servicio con workers: colas de trabajos, consumidores de mensajes, servidores.
- ✅ Despliegues sin pérdida de trabajo (`docker stop`, Kubernetes, `systemd`).
- ❌ Si las tareas pueden repetirse sin problema, un cierre brusco + reintento (arriendos) puede bastar (ver `proyecto-final`).
- ❌ Para un servidor HTTP usa `http.Server.Shutdown`, que ya hace esto.

## Cómo ejecutarlo

```bash
go run .
go test -race -v .
```

## Qué observar (salida real, Apple M1, Go 1.26.5)

```
cierre con gracia suficiente: completo=true, hechas=6 de 6, tardó 300ms
enviar tras cerrar → servicio cerrado
  tarea 0 cancelada por la gracia   (×4, en orden variable)
cierre con gracia insuficiente: completo=false, tardó 200ms
```

## Errores típicos

- **Cerrar el channel mientras otro envía** → pánico. Protégelo (aquí con `RWMutex`) o usa un `ctx` en lugar de `close`.
- Cerrar dos veces: guarda un indicador (`cerrado`) y haz `Cerrar` idempotente.
- Esperar sin gracia: un trabajo colgado bloquea el apagado para siempre.
- Cancelar a los workers **antes** de dejar que drenen: se pierde el trabajo aceptado.
- Olvidar `signal.NotifyContext` en `main`: la señal mata el proceso sin ejecutar nada.

## Profundiza en el curso

- [apagado ordenado](../../../10-errores/06-graceful-shutdown)
- [worker que obedece a Ctrl-C](../../../05-context/07-worker-ctrl-c)
- [apagado de un servidor HTTP](../../../15-diseno-avanzado/10-http-backpressure)
- [apagado del worker con gracia y devolución a la cola](../../../proyecto-final)
- [comprobar que no quedan goroutines](../../../11-testing/05-fugas)

## Prueba tú

1. Añade `signal.NotifyContext` y un `main` que cierre al recibir Ctrl-C.
2. Haz que las tareas pendientes no ejecutadas tras la gracia se devuelvan en un slice.
3. Sustituye el `RWMutex` por un `select` con `ctx.Done()` y compara.
