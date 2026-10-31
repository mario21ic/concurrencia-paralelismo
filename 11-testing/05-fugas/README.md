# Ejercicio 5: detectar goroutine leaks

## Teoría

Una **fuga de goroutine** es una goroutine que nunca terminará (suele estar bloqueada para siempre). Consume memoria, retiene los objetos que referencia y, en un servidor, se acumula hasta degradarlo. El compilador y el runtime **no** avisan: hay que detectarlas.

### Las fugas del ejercicio

| Función | Defecto |
|--|--|
| `ObtenerMal` | si vence el plazo, la goroutine trabajadora se queda bloqueada enviando por un channel **sin buffer** del que ya nadie lee |
| `ObtenerBien` | corregida: channel con **buffer 1**, la goroutine envía y termina |
| `EscuchaMal` | goroutine con `range ch`; si el usuario no cierra `ch`, vive para siempre |

### Cuatro detectores

| Detector | Cómo funciona | Ventajas | Limitaciones |
|--|--|--|--|
| **`goleak`** (`go.uber.org/goleak`) | al terminar el test, compara las pilas de las goroutines vivas con las esperadas (reintenta unos instantes) | pila exacta de cada fuga; muy usado | no está en la stdlib; hay que filtrar goroutines ajenas |
| **Ayudante propio** (`SinFugas`) | `runtime.NumGoroutine()` antes/después y vuelca el perfil `goroutine` | cero dependencias | recuento (ruidoso); no distingue qué goroutine es |
| **`synctest`** | `Test` entra en **pánico** si al acabar la burbuja quedan goroutines bloqueadas | exacto y gratis si ya usas `synctest` | solo detecta lo que corre **dentro** de la burbuja; es un **pánico** (aborta todo el binario de tests), no un fallo normal |
| **Perfil `goroutineleak`** (experimental, Go 1.26) | el **recolector de basura** busca goroutines bloqueadas en recursos **inalcanzables** (nadie podría despertarlas) | detecta fugas **en producción** sin falsos positivos; ignora goroutines bloqueadas pero alcanzables | solo con `GOEXPERIMENT=goroutineleakprofile`; API puede cambiar |

### Matiz de `synctest`: no es solo "fugado", es "pendiente"

`Test` también entra en pánico si el test termina mientras una goroutine **todavía duerme** (`time.Sleep(1h)` virtual). Como el reloj deja de avanzar al salir la goroutine raíz, esa goroutine nunca despertaría. La versión correcta debe **dejar pasar el tiempo** para que el trabajo acabe:

```go
ObtenerBien(trabajo1h, time.Second)   // vence el plazo
time.Sleep(time.Hour)                 // deja que el trabajo termine y salga (buffer: no se bloquea)
```

Con `ObtenerMal` esto **no basta**: tras la hora, la goroutine sigue bloqueada enviando → pánico con `chan send (durable)`.

### Cómo se evita la fuga

1. **Buffer** en el channel de resultado cuando el receptor puede irse (timeout, cancelación).
2. **`select` con `ctx.Done()`** en cada envío/recepción de la goroutine.
3. **Dueño del cierre** de cada channel (módulo 3).
4. **Estructurar** con `errgroup`/`WaitGroup`: nada sobrevive a la función que lo lanzó.

## Qué contiene

- `fugas.go`: `ObtenerMal`, `ObtenerBien`, `EscuchaMal`.
- `verificar.go`: `SinFugas(t)` (ayudante propio).
- `fugas_test.go`: las versiones **correctas** probadas con `goleak`, el ayudante y `synctest`.
- `bugs_test.go` (`-tags bugs`): las versiones con fuga; los tests **fallan**.
- `bugsynctest_test.go` (`-tags bugsynctest`): la fuga detectada por `synctest` (pánico).
- `perfil/main.go`: demostración del perfil `goroutineleak`.

```bash
go test -race -count=20 -timeout 120s .                     # todo en verde
go test -tags bugs -run ObtenerMal_Goleak .                 # goleak
go test -tags bugs -run ObtenerMal_Helper .                 # ayudante
go test -tags bugs -run EscuchaMal .                        # range sin close
go test -tags bugsynctest .                                 # synctest (pánico)
GOEXPERIMENT=goroutineleakprofile go run ./perfil           # perfil goroutineleak
```

## Resultados de referencia

**goleak** (identifica la goroutine y la línea):
```
found unexpected goroutines:
[Goroutine 5 in state chan send, with ...ObtenerMal.func1 on top of the stack:
    .../fugas.go:16
  created by ...ObtenerMal in goroutine 4 ]
```

**Ayudante propio:**
```
fuga de goroutines: antes=2 después=3
goroutine profile: total 3 ...      <- hay ruido: incluye la goroutine que hace el volcado
```

**synctest:**
```
panic: deadlock: main bubble goroutine has exited but blocked goroutines remain
goroutine 22 [chan send (durable), synctest bubble 1]:
    ...ObtenerMal.func1() fugas.go:16
```

**Perfil `goroutineleak`** (2 fugas reales, ignorando la que sigue alcanzable y la correcta):
```
goroutineleak profile: total 2
2 @ ...
#  ...fugas.ObtenerMal.func1+0x37  .../fugas.go:16
(la goroutine bloqueada en `vivo` NO es una fuga: el channel es alcanzable)
```

## Conclusiones

- Los cuatro detectores encuentran la misma fuga, con distinto nivel de detalle y distinto coste.
- **`goleak`**: el estándar en tests unitarios. **`synctest`**: la opción más exacta si el código es temporal. **`goroutineleak`**: la única que sirve en **producción** sin falsos positivos, porque usa la alcanzabilidad.
- Una goroutine **bloqueada** no es lo mismo que una **fugada**: si su channel sigue accesible, podría despertar. El perfil lo distingue (por eso `runtime.KeepAlive(vivo)`).
- `synctest` falla con **pánico**, que aborta todos los tests del paquete: por eso `bugsynctest` está aparte.

## Ejercicios propuestos

1. Arregla `EscuchaMal` para que acepte un `context.Context` y termine con él.
2. Añade `goleak.VerifyTestMain(m)` en un `TestMain` para vigilar todo el paquete.
3. Escribe una fuga que `goleak` detecte pero que el recuento `NumGoroutine` no (una sale y otra entra).
4. Expón el perfil `goroutineleak` en un servidor con `net/http/pprof` y búscalo en `/debug/pprof/goroutineleak`.
