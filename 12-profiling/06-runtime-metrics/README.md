# Ejercicio 6: `runtime/metrics`

## Teoría

`runtime/metrics` es la API **estable y barata** para leer el estado interno del runtime (planificador, GC, memoria, mutex…) desde el propio proceso. Es la base para exportar métricas a Prometheus/OpenTelemetry y para alertas.

```go
s := []metrics.Sample{{Name: "/sched/goroutines:goroutines"}}
metrics.Read(s)
n := s[0].Value.Uint64()
```

- `metrics.All()` lista el catálogo (nombre, descripción, tipo).
- Los nombres tienen unidad: `/gc/heap/live:bytes`, `/sched/latencies:seconds`.
- Tipos de valor: `Uint64`, `Float64` y `Float64Histogram` (cubos acumulativos **desde el arranque**).
- Se pueden leer miles de veces por segundo; no pausa la aplicación.
- Frente a `runtime.ReadMemStats`: `ReadMemStats` **para el mundo** brevemente; `metrics.Read` no.

### Métricas útiles para concurrencia

| Métrica | Qué dice |
|--|--|
| `/sched/goroutines:goroutines` | total de goroutines (¿crecen sin parar? ⇒ fuga) |
| `/sched/goroutines/{running,runnable,waiting,not-in-go}` | desglose por estado. **`runnable` alto ⇒ más trabajo que núcleos** |
| `/sched/latencies:seconds` | histograma: tiempo que una goroutine **lista** espera hasta tener CPU |
| `/sync/mutex/wait/total:seconds` | tiempo acumulado esperando mutex (derivada ⇒ cuánta contención) |
| `/sched/gomaxprocs:threads` | `GOMAXPROCS` efectivo |
| `/cpu/classes/{user,idle,gc/total,total}:cpu-seconds` | reparto de la CPU; el GC compite con tu código |
| `/gc/cycles/total:gc-cycles`, `/gc/heap/live:bytes` | frecuencia del GC y tamaño del conjunto vivo |

### Histogramas

`Buckets[i]` es el límite **inferior** del cubo `i` (el superior es `Buckets[i+1]`); `Counts[i]` cuántas observaciones. Como son **acumulativos desde el arranque**, para medir un intervalo hay que **restar** dos fotos (`RestarHistograma`). Un cuantil solo se conoce como **cota superior del cubo** (`Cuantil`), y los cubos son anchos a magnitudes altas: **un p99 de "2.7 s" es "entre 1.3 s y 2.7 s"**, no un valor exacto.

### Advertencias

- Las métricas de `/cpu/classes/*` **se actualizan al final de cada ciclo de GC**. Si el programa no asigna, no hay ciclos y las lecturas no cambian (en este ejercicio se fuerza `runtime.GC()` antes de leerlas). En un servicio normal se actualizan solas.
- Un `runtime.GC()` forzado **para el mundo**: se hace *después* de tomar el histograma de latencias para no contaminarlo.
- Derivadas, no valores absolutos: casi todo (mutex, ciclos, CPU) es un **contador acumulativo**; lo interesante es su **tasa** (diferencia entre dos lecturas).

## Qué hace el programa

Cuatro escenarios de 1 s en una máquina de 8 núcleos, cada uno medido con fotos antes/después y un muestreador cada 20 ms:

| Escenario | Qué provoca |
|--|--|
| **A)** 8 goroutines de CPU | carga ajustada a los núcleos |
| **B)** 256 goroutines de CPU (32×) | saturación del planificador |
| **C)** 8 goroutines peleando por un mutex | contención (sección crítica de 200 µs) |
| **D)** 2000 goroutines bloqueadas | muchas goroutines, ninguna con trabajo |

```bash
go run .
go test -race .
```

## Resultados de referencia (3 ejecuciones; las columnas con rango varían)

```
escenario                              gor.máx  listas  espera.   lat p50     lat p99     mutex(s)  CPU %
A) 8 goroutines de CPU (= núcleos)          23     2-4      14     1-2 µs     57-655 µs      0.00    100%
B) 256 goroutines de CPU (32× núcleos)     271   252-254    14   335-671 ms   0.9-6.4 s      0.00    100%
C) 8 goroutines peleando por un mutex       23     2-3      21     1-3 µs     33-49 µs     7.0-8.3     14%
D) 2000 goroutines bloqueadas              2015    0-1    2014     3-7 µs     41-82 µs       0.01       0%
```

## Conclusiones

- **A frente a B:** con tantas goroutines como núcleos, una goroutine lista consigue CPU en **microsegundos**; con 32× núcleos hay **~252 listas** a la vez y la espera mediana pasa a **cientos de milisegundos**. Es aritmética del planificador: 32 goroutines por núcleo × franjas de 10 ms ≈ 320 ms entre turnos (se observó 335 ms). Las dos usan el 100 % de la CPU: **la utilización no distingue una de otra; `runnable` y la latencia de planificación sí**.
- **C:** la CPU está al **14 %** (≈ 1 de 8 núcleos): el mutex serializa. Y `/sync/mutex/wait/total` crece **7-8 segundos por cada segundo** de reloj, es decir, ~7-8 goroutines esperando de media. Es el patrón "la CPU está ociosa y nada avanza".
- **D:** 2014 goroutines `waiting` y 0 % de CPU: muchas goroutines **no son un problema por sí mismas**; son baratas mientras estén bloqueadas.
- Alertas útiles que se derivan: `runnable > k·GOMAXPROCS` sostenido, `p99 de latencia de planificación`, `d(mutex/wait)/dt` alto, `goroutines` creciendo sin techo.

## Tests

`Cuantil` (con un histograma construido a mano), `RestarHistograma` y que `Leer` devuelva datos coherentes.

## Ejercicios propuestos

1. Exporta `Leer()` como endpoint `/metrics` en formato texto de Prometheus.
2. Añade un escenario "E": 8 goroutines que asignan mucha memoria y observa `/cpu/classes/gc/total` y `/gc/cycles/total` sin forzar `runtime.GC()`.
3. Calcula el *p99* de `/sched/pauses/total/gc:seconds` (pausas de GC).
4. Repite el escenario B con `GOMAXPROCS=2` y compara la latencia de planificación.
