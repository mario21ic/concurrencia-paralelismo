# Ejercicio 8: diagnóstico integrador

## Enunciado

Un servicio HTTP con **tres problemas ocultos**, uno por endpoint. Encuéntralos con las herramientas del módulo, corrígelos y **demuestra la mejora con números**.

| Endpoint | Síntoma que verá un usuario | Causa (no mires el código aún) |
|--|--|--|
| `/cpu` | lento, usa mucha CPU | ? |
| `/mutex` | latencia alta con carga, CPU ociosa | ? |
| `/fuga` | rápido… pero la memoria crece | ? |

## Método: de síntoma a herramienta

```
¿la CPU está al máximo?      ── sí ──► perfil de CPU (-top, -list)
        │ no
¿la latencia es alta con CPU libre? ──► perfil de mutex / block / traza  (alguien espera)
        │
¿memoria o goroutines crecen sin parar? ──► perfil de goroutine (debug=1) y de heap
        │
¿ocurre solo a veces? ──► flight recorder (ejercicio 5)
```

## Cómo ejecutarlo

```bash
# terminal 1: el servicio (con pprof expuesto en /debug/pprof/ y perfil de mutex activado)
go run . -modo=servidor -addr 127.0.0.1:18090

# terminal 2: generar carga (6 clientes por endpoint, 6 s)
go run . -modo=carga -url http://127.0.0.1:18090 -dur 6s

# terminal 3: mientras corre la carga
go tool pprof -top -nodecount=12 'http://127.0.0.1:18090/debug/pprof/profile?seconds=3'   # CPU
go tool pprof -top -cum http://127.0.0.1:18090/debug/pprof/mutex                          # contención
curl -s 'http://127.0.0.1:18090/debug/pprof/goroutine?debug=1' | head -1                  # nº de goroutines
go tool pprof -top http://127.0.0.1:18090/debug/pprof/goroutine                           # dónde están

# comprobar la corrección
go run . -modo=servidor -addr 127.0.0.1:18090 -arreglado
```

> `go tool pprof` descarga y **guarda** los perfiles obtenidos por URL en `~/pprof/`. Bórralos cuando termines.

## Diagnóstico (resultados reales de esta ejecución)

**Carga con los tres problemas:**

```
endpoint peticiones      req/s        p50        p99
/cpu          30177       5030      960µs     5.47ms
/mutex        10118       1686     3.15ms    12.17ms
/fuga         32466       5411      830µs     5.19ms
```

### 1) `/cpu` — perfil de CPU

```
Duration: 3.11s, Total samples = 15080ms (484.33%)
 flat%   cum%
 26.13%  syscall.rawsyscalln         <- red: es el coste normal del servidor
  5.84%  strings.ToUpper  (13.4 % cum)
  3.71%  crypto/.../sha256.blockSHA2
  3.18%  fmt.(*pp).doPrintf (10.7 % cum)
  2.12%  runtime.mallocgc (11.4 % cum)
```

`-list 'main.servidor.func1'` (la línea exacta):

```
1.50s   s := strings.ToUpper(fmt.Sprintf("usuario-%d-%s", i, "region"))     // línea 81
500ms   h := sha256.Sum256([]byte(s))                                        // línea 82
```

**Hallazgo:** el handler repite el mismo cálculo (con muchas asignaciones) 2000 veces por petición. **Corrección:** hacerlo una sola vez, con un búfer reutilizado.

### 2) `/mutex` — perfil de mutex

```
Type: delay     Total: 25540 ms de espera acumulada en 6 s
cum
24806ms 97.1%  main.servidor.func2          <- el handler de /mutex
```

La CPU del servicio no estaba saturada pero `/mutex` tardaba 3 ms de mediana para 0.5 ms de trabajo: el resto era esperar el lock global. **Corrección:** el trabajo (`busy`) se mueve **fuera** de la sección crítica y el contador pasa a `atomic`.

### 3) `/fuga` — perfil de goroutines

```
goroutine profile: total 32464          <- (3 al arrancar)
```

En una ejecución anterior de la misma prueba, el desglose por función (`go tool pprof -top …/goroutine`) mostraba **36 776 de 36 777** goroutines en `main.servidor.func3.1` → `runtime.chansend1` → `runtime.gopark`: todas bloqueadas en un **envío** a un channel.

En la ejecución documentada aquí hay 32 464 goroutines, creadas por el handler de `/fuga`: casi **una por petición** (se atendieron 32 466). La goroutine de "auditoría" tarda más que el plazo del handler, y el handler ya se fue: nadie recibe. **Corrección:** channel con buffer de 1.

## Después de la corrección

```
endpoint peticiones      req/s        p50        p99
/cpu         126405      21068      150µs     1.89ms        <- 4.2× más req/s; p99 5.47 → 1.89 ms
/mutex        40606       6768      690µs     3.79ms        <- 4.0× más req/s; p99 12.17 → 3.79 ms
/fuga         32625       5438      870µs     4.19ms        <- sin cambio de rendimiento

goroutines tras la carga:  32464  →  3
espera acumulada de mutex: 25540 ms  →  381 ms
```

## Conclusiones

- **Cada herramienta ve un tipo de problema distinto.** El perfil de CPU no habría encontrado `/mutex` (la CPU estaba libre) ni `/fuga` (las goroutines bloqueadas no consumen CPU).
- **La fuga no se nota en rendimiento ni latencia:** `/fuga` rindió igual antes y después (5411 vs 5438 req/s). Solo la **cuenta de goroutines** (y la memoria) la delatan; por eso hay que **vigilar `/sched/goroutines`** como métrica y alertar si crece sin techo. A mayor escala acaba en OOM, no en lentitud.
- **Corregir sin medir no es optimizar**: cada arreglo se valida con la misma carga y con la misma herramienta que lo encontró.
- Los perfiles se sacan **en caliente, sin parar el servicio** (`/debug/pprof`): es la ventaja de tener `net/http/pprof` listo desde el principio (en un puerto interno).

## Ejercicios propuestos

1. Añade un cuarto problema (un `select` sin salida, o un `time.After` en bucle) y encuéntralo con el perfil adecuado.
2. Activa también `SetBlockProfileRate` y compara el perfil `block` con el de `mutex` para `/mutex`.
3. Captura una traza de 2 s (`/debug/pprof/trace?seconds=2`) bajo carga y analízala con `go tool trace -pprof=sync`.
4. Exporta `/sched/goroutines` y `/sync/mutex/wait/total` (ejercicio 6) en un endpoint `/metrics`.
5. Escribe un test que arranque el servicio con `httptest`, lance 1000 peticiones a `/fuga` y compruebe con `goleak` que no hay fuga (módulo 11).
