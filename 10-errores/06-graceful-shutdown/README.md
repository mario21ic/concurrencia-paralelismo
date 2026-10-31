# Ejercicio 6: apagado ordenado de un servidor HTTP

## Teoría

Un servicio que se detiene bruscamente pierde peticiones en curso (el cliente recibe un error de conexión), deja trabajo a medias y puede corromper estado. El **apagado ordenado** (*graceful shutdown*) sigue una secuencia:

1. Llega la señal (`SIGINT`/`SIGTERM`) → se cancela el contexto raíz.
2. Se **deja de aceptar** conexiones nuevas.
3. Se **espera** a las peticiones en curso, hasta un plazo de gracia.
4. Pasado el plazo, se **fuerza** el cierre.
5. Se detienen los workers de fondo y se liberan recursos.

### `http.Server`

| Método | Efecto |
|--|--|
| `Shutdown(ctx)` | cierra los listeners, cierra conexiones ociosas y **espera** a las activas; devuelve `ctx.Err()` si vence el plazo |
| `Close()` | corta **todas** las conexiones inmediatamente |
| `Serve(ln)` | devuelve `http.ErrServerClosed` tras `Shutdown`/`Close`: es el cierre esperado, no un fallo |

`Shutdown` **no cancela** el `r.Context()` de las peticiones en curso: las deja terminar. Tras `Close()`, las conexiones se cortan y su contexto se cancela.

### Diseño de `Ejecutar`

Tres componentes en un `errgroup.WithContext`:

```
gctx ──┬── Serve(ln)        → termina con ErrServerClosed (nil) o con un fallo real
       ├── apagador         → espera gctx.Done(), luego Shutdown(gracia) / Close()
       └── worker de fondo  → ticker hasta gctx.Done()
```

Puntos clave:
- El apagador espera `gctx.Done()`, no `ctx.Done()`: así se activa **también** si otro componente falla (p. ej. el listener muere), y el resto se detiene de forma coordinada.
- El plazo de gracia usa `context.WithoutCancel(ctx)`: el contexto ya está cancelado cuando empieza el apagado; si el plazo lo heredara, `Shutdown` abortaría al instante.
- `Ejecutar` devuelve `nil` si todo fue limpio; un error si se agotó la gracia o falló un componente.
- En `main`, tras la primera señal se llama a `stop()` para restaurar el comportamiento por defecto: una **segunda** `Ctrl+C` fuerza la salida.

### En Kubernetes

Orden real: se quita el pod del Service → `SIGTERM` → `terminationGracePeriodSeconds` (30 s por defecto) → `SIGKILL`. La gracia de tu proceso debe ser **menor**. Además, el balanceador puede seguir enviando tráfico unos segundos tras el `SIGTERM`: un pequeño retardo (*preStop*) antes de `Shutdown` ayuda.

## Cómo ejecutarlo

```bash
go run . -addr 127.0.0.1:8080                  # Ctrl+C para apagar
go run . -duracion 3s                          # se apaga sola
curl localhost:8080/lento &                    # tarda 2 s
kill -TERM <pid>                               # la petición en curso termina igualmente
go test -race -timeout 60s .
```

## Resultado de referencia (prueba real con `kill -TERM` mientras `/lento` estaba en curso)

```
nueva conexión durante el apagado: (curl exit=7)      <- rechazada: ya no acepta
respuesta de la petición en curso: listo (tras 2s)    <- ¡completada!

log del servidor:
escuchando en 127.0.0.1:18089 (pid 78845)
apagando... (otra señal fuerza la salida)
worker de fondo detenido
apagado limpio
```

## Tests (`servidor_test.go`)

| Test | Verifica |
|--|--|
| `PeticionEnCursoTerminaTrasElApagado` | una petición de 300 ms iniciada antes del apagado se completa con su respuesta; `Ejecutar` devuelve `nil`; luego se rechazan conexiones |
| `GraciaAgotadaCortaConexiones` | con gracia de 100 ms y una petición de 5 s, `Ejecutar` devuelve error, el cliente recibe un error y el apagado no tarda más de 2 s |
| `WorkerDeFondoSeDetiene` | el worker latió mientras vivía y dejó de latir al apagarse |
| `FalloDelListenerCancelaAlResto` | si `Serve` falla, el apagador y el worker también terminan |

## Conclusiones

- La petición en curso terminó con éxito y las nuevas se rechazaron: ese es el objetivo del apagado ordenado.
- El plazo de gracia tiene dos finales posibles (limpio o forzado) y ambos están probados.
- Un solo `errgroup` hace que cualquier fallo detenga todo el servicio de forma coordinada.

## Ejercicios propuestos

1. Añade un endpoint `/salud` que devuelva 503 en cuanto empiece el apagado (para que el balanceador deje de enviar tráfico).
2. Registra cuántas peticiones estaban en curso al empezar el apagado (contador atómico en un middleware).
3. Añade un componente que, al apagarse, vacíe un buffer a disco (con su propio plazo).
4. Sustituye `http.Server` por un servidor gRPC (`GracefulStop`) manteniendo el mismo esquema.
