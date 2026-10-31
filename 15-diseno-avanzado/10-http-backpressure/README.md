# Ejercicio 10: concurrencia en servidores `net/http`

## Teoría

### El modelo: una goroutine por conexión, un handler por solicitud

`http.Server.Serve` acepta conexiones en un bucle y lanza **una goroutine por conexión**; en cada una atiende las solicitudes (HTTP/1.1 secuencialmente; HTTP/2 lanza una goroutine por *stream*). Tus handlers se ejecutan **concurrentemente** ⇒ todo el estado compartido (cachés, contadores, mapas) necesita sincronización (módulos 6-7), y **no hay límite por defecto**: 100 000 clientes ⇒ 100 000 goroutines (≈ 2-8 KB cada una más búferes de E/S: sí cabe en memoria, pero el recurso *detrás* (BD, CPU, API externa) no).

> Las goroutines son baratas; **lo que hacen no lo es**. El límite hay que ponerlo donde está el recurso escaso.

### Sobrecarga: qué ocurre sin control

Si llega más trabajo del que el sistema puede procesar, la cola crece (en memoria, en el kernel, en el balanceador), la **latencia sube** hasta que los clientes se rinden por *timeout*… y el servidor **sigue trabajando** para respuestas que nadie espera. Resultado: el *throughput útil* se **hunde** justo cuando más se necesita (colapso por sobrecarga, *congestion collapse*).

| Técnica | Qué hace | Efecto |
|--|--|--|
| **Propagar `r.Context()`** | cuando el cliente cierra la conexión o vence el plazo, el contexto se cancela; el handler debe dejar de trabajar | evita trabajo inútil |
| **Límite de concurrencia + descarte (503)** | semáforo no bloqueante; si no hay plaza, `503` + `Retry-After` al instante (*load shedding*) | latencia de las admitidas estable; el cliente se entera ya |
| **Cola acotada con plazo** | unas pocas esperan, el resto se rechaza; espera máxima corta | absorbe ráfagas sin disparar la latencia |
| **Timeouts del servidor** | `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`, `MaxHeaderBytes`, `http.MaxBytesReader` | un cliente lento/malicioso (*slowloris*) no retiene goroutines ni descriptores |
| **Apagado ordenado** | `Shutdown(ctx)`: deja de aceptar, espera a las solicitudes en curso | despliegues sin errores 5xx |
| **Reintentos con *backoff* + *jitter*** (lado cliente) | tras un `503`, espera aleatoria | evita la *estampida* sincronizada |

### Backpressure

*Backpressure* es **comunicar la saturación hacia atrás** (al cliente) en vez de acumularla. En HTTP: `503 Service Unavailable` / `429 Too Many Requests` con `Retry-After`. Fallar pronto y barato es **mejor** que aceptar y responder tarde: una respuesta tardía suele ser inútil y consume recursos que podrían servir a otra.

### Dónde poner el límite

Idealmente **en el recurso que se agota** (el pool de conexiones a la BD, un semáforo por API externa), no solo en el borde. El límite de entrada (`Limitar`) es la red de seguridad general; un semáforo cerca del recurso (con `ctx`) es la defensa precisa. Y calcula el valor: por la **ley de Little** `concurrencia = tasa × latencia`; con 8 plazas de 20 ms, la capacidad es 400 op/s.

### Apagado ordenado (`Servir`)

1. `ctx` cancelado (señal `SIGTERM` con `signal.NotifyContext`).
2. `srv.Shutdown(ctx con gracia)`: cierra los *listeners*, espera a que acaben las solicitudes activas y a que las conexiones ociosas se cierren.
3. Si se agota la gracia, `srv.Close()` corta lo que quede.
Las goroutines que lance tu handler y sobrevivan a la solicitud **no** las espera `Shutdown`: ¡tienen que estar atadas a un `WaitGroup`/errgroup propio (ejercicio 9)!

## Qué hace el programa

| Archivo | Contenido |
|--|--|
| `servidor.go` | `Backend` (8 plazas × 20 ms), handlers que ignoran o respetan el contexto, `Limitar`, `LimitarConCola`, `NuevoServidor` (timeouts), `Servir` (apagado ordenado) |
| `main.go` | 200 clientes en bucle cerrado durante 2 s contra cuatro configuraciones; los clientes se rinden a los 300 ms y, tras un 503, esperan 20 ms |
| `servidor_test.go` | `httptest`: el límite nunca se supera, `Retry-After`, la cola absorbe y acota, la cancelación del cliente llega al handler, apagado ordenado, *slowloris* cortado, y `goleak` |

```bash
go run .
go test -race -v .
```

## Resultados de referencia (Apple M1, loopback, Go 1.26.5)

```
backend: 8 plazas × 20 ms ⇒ capacidad 400 op/s. 200 clientes durante 2 s; los clientes se rinden a los 300 ms.

  configuración                                  ok/s  rechaz. caducad.  p50 ok  p99 ok  inútil  goroutines (pico)
  A) sin límite, el handler ignora ctx             49        0     1413   143ms   285ms    1364    1512
  B) sin límite, el handler respeta r.Context()   137        0     1158   300ms   303ms     302    1022
  C) límite 8 + 503 inmediato                     366    17033       38    20ms    54ms       0     823
  D) límite 8 + cola 16 (espera máx. 100 ms)      391    16833        0    61ms    69ms       0     778
```
`inútil` = operaciones del backend terminadas cuyo resultado no recibió nadie. Las goroutines incluyen las 200 del generador de carga y las de su cliente HTTP.

## Conclusiones

- **El colapso es real:** sin control (A) el servidor hace ~1 360 operaciones inútiles y sirve **49 ok/s: 8× menos que su capacidad de 400**. Trabaja a pleno rendimiento… para clientes que ya se fueron.
- **Respetar `r.Context()` (B) ya ayuda mucho** (137 ok/s, 4.5× menos trabajo inútil, 302), pero la latencia sigue pegada al plazo (p50 300 ms): hay demasiados en cola.
- **Limitar y descartar (C) casi alcanza la capacidad: 366 ok/s (92 %)**, con p50 de 20 ms —lo que cuesta la operación— y **cero trabajo inútil**. Los 17 033 rechazos son baratos y rápidos: es el "backpressure" funcionando (los clientes sintéticos reintentan cada 20 ms).
- **Una cola corta (D) sube a 391 ok/s (98 %)** a cambio de latencia (p50 61 ms) y **cero** solicitudes caducadas: absorbe los huecos sin acumular espera larga.
- **Límite y latencia van de la mano:** a mayor cola, más throughput hasta un punto, pero la espera crece; el plazo máximo en la cola (`espera`) pone el tope a esa latencia.
- **Los timeouts del servidor son defensa, no optimización:** el test del *slowloris* confirma que `ReadHeaderTimeout` cierra la conexión lenta; sin él, cada conexión así retiene una goroutine y un descriptor.

## Ejercicios propuestos

1. Añade a `Limitar` un límite **por cliente** (IP o API key) con `golang.org/x/time/rate` y devuelve `429`.
2. Implementa un **semáforo adaptativo** (AIMD): sube el límite mientras la latencia sea baja y bájalo al ver errores.
3. Integra `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)` y prueba con `kill -TERM` que las solicitudes en curso terminan.
4. Usa `http.TimeoutHandler` y compara con `Limitar`: ¿qué trabajo se desperdicia en cada caso?
