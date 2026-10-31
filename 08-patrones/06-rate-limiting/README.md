# Ejercicio 6: limitación de tasa

## Teoría

Un **rate limiter** limita cuántas operaciones ocurren **por unidad de tiempo** (p. ej. 100 peticiones/s a una API de terceros, o por usuario).

### Tres enfoques

| Enfoque | Cómo | Ráfagas | Cuándo |
|--|--|--|--|
| **Ticker** | un tick = una operación | no | tasa fija y simple |
| **Token bucket** | cubo con capacidad `burst`, se rellena a `rate` tokens/s; cada operación consume 1 | **sí**, hasta `burst` | el estándar; tolera picos |
| **Leaky bucket / ventana** | cola que "gotea" a ritmo fijo / contador por ventana | no / sí | suavizar salida / límites por minuto |

### Token bucket

- `rate`: velocidad sostenida. `burst`: tamaño máximo de ráfaga.
- Con el cubo lleno puedes hacer `burst` operaciones seguidas; después, una cada `1/rate` s.
- **Relleno perezoso:** no hace falta una goroutine que lo llene. Al consultar se calcula `tokens += tiempo_transcurrido × rate` (acotado por `burst`). El reloj se inyecta para probarlo sin dormir.

### API de `x/time/rate`

| Método | Efecto |
|--|--|
| `Allow()` | ¿puedo ahora? no bloquea (devolver HTTP 429) |
| `Wait(ctx)` | espera al token; **falla de inmediato** si el plazo del `ctx` no alcanza |
| `Reserve()` | reserva y dice cuánto esperar (`Delay()`); `Cancel()` devuelve el token |
| `SetLimit`, `SetBurst` | cambiar en caliente |
| `rate.Every(d)` | convierte un intervalo en `rate.Limit` |

Para límites **por clave** (usuario, IP) se guarda un `*rate.Limiter` por clave en un mapa protegido, con expiración de los inactivos.

### Cuidado

- El limitador local limita **un proceso**. Para límites globales entre réplicas hace falta un almacén compartido (Redis, etc.).
- `Wait` de mi implementación no garantiza **FIFO** entre varios esperando; el de `x/time/rate` usa reservas, que sí ordenan.
- Rate limit ≠ concurrencia máxima: 10 ops/s con 5 s de latencia son 50 en vuelo.

## Qué hace el programa

1. Ticker: una operación cada 100 ms.
2. Token bucket propio: 10/s, burst 3.
3. `x/time/rate` con la misma configuración.
4. `Allow` (2/s, burst 2): rechaza lo que sobra.
5. `Reserve`: cuánto habría que esperar.
6. `Wait` con plazo insuficiente.

```bash
go run .
go test -race -count=3 .
```

## Resultado de referencia

```
== 2) token bucket propio: 10/s, burst 3 ==
  op 1 a los 0s
  op 2 a los 0s
  op 3 a los 0s        <- ráfaga de 3
  op 4 a los 100ms     <- luego 1 cada 100 ms
  op 5 a los 200ms
  ...
  op 8 a los 500ms

== 3) x/time/rate: idéntico ==

== 4) Allow (no bloquea): 2/s, burst 2 ==
  petición 1 permitida: true
  petición 2 permitida: true
  petición 3 permitida: false
  petición 4 permitida: false
  tras 520ms, permitida: true

== 5) Reserve: habría que esperar ~200ms ==
== 6) Wait con plazo de 100ms (el siguiente token tarda 1s) ==
  rate: Wait(n=1) would exceed context deadline
```

## Conclusiones

- El ticker no permite ráfagas; el token bucket sí, y a largo plazo converge a la misma tasa.
- La implementación propia (≈ 40 líneas) reproduce exactamente el comportamiento de `x/time/rate`.
- `Wait` con un plazo insuficiente devuelve el error **al instante** en vez de esperar hasta el plazo: útil para fallar rápido.

## Tests

Con **reloj falso**: ráfaga y agotamiento, relleno exacto (a los 99 ms no, a los 100 ms sí), el cubo nunca supera `burst` aunque pase una hora, y `Wait` cancelable.

## Ejercicios propuestos

1. Límite por usuario: `map[string]*rate.Limiter` con limpieza de inactivos.
2. Middleware HTTP que responda 429 con `Retry-After` usando `Reserve().Delay()`.
3. Combina rate limit + semáforo para un cliente de API (X por segundo y máximo Y simultáneas).
4. Implementa una ventana deslizante y compárala con el token bucket.
