# Ejercicio 1: interbloqueo, livelock, inanición e inversión de prioridad

## Teoría

Son cuatro formas distintas de **no progresar**. Se distinguen por *qué hacen* las goroutines mientras no avanzan:

| Fallo | Qué ocurre | ¿Consume CPU? | ¿Lo detecta el runtime? |
|--|--|--|--|
| **Deadlock** | un ciclo de goroutines, cada una espera algo que tiene otra del ciclo | no (bloqueadas) | **solo si TODAS** las goroutines están bloqueadas (`fatal error: all goroutines are asleep`) |
| **Livelock** | las goroutines se mueven pero sin avanzar: cada una reacciona a la otra (ceden, reintentan…) | **sí** | no |
| **Starvation** (inanición) | el sistema avanza, pero una goroutine concreta nunca consigue el recurso | no/sí | no |
| **Inversión de prioridad** | una tarea urgente espera a una de baja prioridad que a su vez no avanza porque otras de prioridad media la desplazan | no | no |

### Las 4 condiciones de Coffman (deadlock)

Hace falta que se den **las cuatro a la vez**; romper **una** basta:

1. **Exclusión mutua** (el recurso no es compartible).
2. **Mantener y esperar** (retengo uno mientras pido otro).
3. **Sin expropiación** (no se me puede quitar el recurso).
4. **Espera circular** (A espera a B, B espera a A).

| Técnica | Condición que rompe | Coste |
|--|--|--|
| **Orden global de adquisición** (por ID, dirección…) | espera circular | hay que conocer *todos* los recursos antes; disciplina de equipo |
| **`TryLock` + plazo y soltar** | mantener y esperar | puede fallar → exige reintento con *jitter* (si no, livelock) |
| Pedir todos los recursos de golpe (un solo cerrojo grueso) | mantener y esperar | menos concurrencia |
| Evitar tener 2 cerrojos (propiedad de una sola goroutine, channels) | exclusión mutua | rediseño |

### Detectar un deadlock en Go

- Si **todas** las goroutines están bloqueadas, el runtime aborta con `all goroutines are asleep - deadlock!`.
- En un servidor real **siempre hay** alguna goroutine viva (un `http.Server`, un ticker…) ⇒ **el runtime nunca avisa**. Hay que detectar el síntoma: *vigilante de progreso* (contador que debe avanzar), `pprof` (`/debug/pprof/goroutine?debug=2`), `kill -SIGQUIT`, o tiempos máximos (`context.WithTimeout`).
- `go vet` y `-race` **no** detectan deadlocks. El *race detector* de Go tampoco detecta inversión de orden de cerrojos (a diferencia de ThreadSanitizer en C++).

### Livelock

Ejemplo humano: dos personas en un pasillo que se apartan a la vez hacia el mismo lado, una y otra vez. En código: `TryLock`, falla, suelta el primero, **reintenta al instante**, y la otra goroutine hace lo mismo. Se rompe con **asimetría**: retardo **aleatorio** (*jitter*) o exponencial, prioridades por ID.

### Inanición

Aparece cuando el recurso se concede por una política **injusta** (prioridad, `sync.Mutex` en modo normal, lector-preferente…). Remedios: **justicia (FIFO)**, **envejecimiento** (subir la prioridad con la espera) o cuotas.

### Inversión de prioridad

Go **no tiene prioridades de goroutine**, pero la inversión aparece igual en cualquier recurso *de la aplicación* con política de prioridad (una cola de trabajos, un pool de conexiones, un semáforo ponderado) combinado con un lock. Caso clásico (Mars Pathfinder, 1997): tarea **baja** tiene el cerrojo, la **alta** lo necesita y las **medias** acaparan la CPU, así que la baja nunca suelta el cerrojo.

Soluciones: **herencia de prioridad** (el propietario hereda la del que espera), **techo de prioridad**, o no compartir cerrojos entre prioridades diferentes.

## Qué hace el programa

| Archivo | Contenido |
|--|--|
| `tecnicas.go` | `TransferirMal` (deadlock), `TransferirOrdenado`, `TransferirConPlazo`, `Reintentos` (livelock con/sin *backoff*) |
| `prio.go` | `PrioSem` (semáforo por prioridad con envejecimiento opcional) y `Mutex` con herencia de prioridad opcional |
| `escenarios.go` | `Inversion` y `Inanicion` sobre una "CPU" de un hueco con turnos de 2 ms |
| `main.go` | recorre A-E |

```bash
go run .                         # A-E
go run . -modo=deadlock-total    # el runtime lo detecta y aborta
go test -race -v .
```

## Resultados de referencia

```
== A) deadlock PARCIAL: el runtime NO lo detecta si hay otras goroutines vivas ==
  tras 300 ms: transferencias completadas = 0 de 2 (el runtime sigue sin quejarse)
    1 goroutine(s): bloqueada en main.TransferirMal (tecnicas.go:23)  (×2)

== B) dos soluciones: orden global de cerrojos y TryLock+jitter → ambas completan, saldos 200 = 200

== C) livelock (TryLock + soltar + reintentar al instante)
  2 goroutines: sin backoff 216480 fallos en 6.68s | con backoff aleatorio   1 fallos en 1ms
  8 goroutines: sin backoff  56225 fallos en 1.7 s | con backoff aleatorio  17 fallos en 1ms

== D) inversión de prioridad
  sin herencia: la tarea ALTA tarda 156ms
  con herencia: la tarea ALTA tarda  23ms   (6.9× menos)

== E) inanición
  sin envejecimiento: espera 364ms
  con envejecimiento (+1 prio cada 5 ms): espera 52ms
```

## Conclusiones

- **El deadlock parcial es silencioso:** tras 300 ms 0 de 2 transferencias, sin ningún aviso. El volcado de goroutines (`pprof`) señala exactamente las dos líneas donde están bloqueadas. Con `wg.Wait()` en `main` y sin más goroutines, en cambio, el runtime aborta.
- **Orden global de cerrojos** es la solución más barata y robusta *cuando se puede aplicar*; `TryLock` es el plan B, pero **exige jitter** o cambia un deadlock por un livelock.
- **El livelock gasta CPU sin avanzar:** 216 480 intentos fallidos (6.7 s) frente a 1 con backoff aleatorio (1 ms). La solución es **romper la simetría**, no "reintentar más rápido".
- **La inversión de prioridad multiplicó x6.9 la latencia de la tarea urgente** (156 ms frente a 23 ms): esperaba no solo a la baja sino a las 10 medias. Con herencia solo esperó los 5 turnos de la baja.
- **Sin envejecimiento, la tarea baja esperó 364 ms** (a que acabaran los flujos urgentes); con envejecimiento, 52 ms. La prioridad estricta es injusta por diseño.
- La herencia aquí es una **versión didáctica**: tiene una ventana (propietario todavía sin registrar) que un `pthread_mutex` con `PTHREAD_PRIO_INHERIT` resuelve en el kernel.

## Ejercicios propuestos

1. Genera un deadlock entre `sync.RWMutex` (lector que pide un segundo `RLock` recursivo con un escritor esperando) y explica por qué ocurre.
2. Añade un detector de ciclos al estilo *lock graph*: registra qué goroutine tiene y espera cada cerrojo y avisa si encuentra un ciclo.
3. Sustituye el jitter uniforme por **backoff exponencial** con tope y compara los fallos.
4. Implementa **techo de prioridad** (*priority ceiling*) en `Mutex` y compara con la herencia.
