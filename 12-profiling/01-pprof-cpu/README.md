# Ejercicio 1: perfil de CPU con `pprof`

## Teoría

Un **perfil de CPU** responde a "¿dónde gasta tiempo de procesador mi programa?". El runtime interrumpe el proceso unas **100 veces por segundo** (`SIGPROF`) y anota la pila de cada goroutine que está ejecutando. Con suficientes muestras, las funciones que aparecen más veces son las que más CPU consumen.

### Cómo obtenerlo

| Forma | Cuándo |
|--|--|
| `pprof.StartCPUProfile(f)` / `StopCPUProfile()` | en un programa de duración limitada (este ejercicio) |
| `go test -bench . -cpuprofile cpu.out` | al optimizar una función concreta |
| `net/http/pprof` → `/debug/pprof/profile?seconds=30` | en un servicio en marcha (ejercicio 8) |

### Cómo leerlo

```bash
go tool pprof -top -nodecount=15 cpu.pprof          # resumen
go tool pprof -top -cum cpu.pprof                    # ordenado por tiempo acumulado
go tool pprof -list 'main.claveLenta' cpu.pprof      # tiempo POR LÍNEA de una función
go tool pprof -http=:8081 cpu.pprof                  # interfaz web (grafo, llama, fuente)
```

| Columna | Significado |
|--|--|
| **flat** | tiempo gastado *en la propia función* |
| **cum** | tiempo en la función **y en todo lo que llama** |
| `flat%`, `cum%` | porcentaje sobre el total de muestras |

- *flat* alto ⇒ el punto caliente está **dentro** de esa función.
- *cum* alto con *flat* bajo ⇒ la función es un "camino" hacia el punto caliente (mira a sus llamados).

### Perfilar código concurrente

- La cabecera `Total samples = 2800ms (398%)` en una ejecución de `700 ms` **no es un error**: con 8 núcleos ocupados el perfil acumula hasta 800 ms de CPU por cada 100 ms de reloj. Es una medida de **CPU consumida**, no de tiempo transcurrido.
- Un perfil de CPU **no ve** el tiempo que las goroutines pasan **bloqueadas** (esperando un lock, un channel, la red). Para eso están los perfiles *mutex*, *block* y la traza (ejercicios 2 y 4).
- Muchas asignaciones se pagan en el runtime (`mallocgc`, `madvise`, `usleep`, bloqueos del asignador, GC), y esas funciones **ocultan al culpable**: hay que subir por la pila con `-cum`/`-list` hasta el código propio.

## Qué hace el programa

La misma tarea (generar una clave, aplicar SHA-256 y codificar en hexadecimal) en 5 000 000 de elementos repartidos entre 8 workers, de dos formas con **resultado idéntico** (misma suma de comprobación):

- **lento:** `fmt.Sprintf` + `strings.ToUpper` + conversiones `[]byte`/`string` + `hex.EncodeToString`: cinco asignaciones por elemento.
- **rápido:** búferes reutilizados por worker (`strconv.AppendInt`, `hex.Encode`): cero asignaciones.

```bash
go run . -modo=lento  -perfil lento.pprof
go run . -modo=rapido -perfil rapido.pprof
go tool pprof -top -nodecount=10 lento.pprof
go tool pprof -list 'main.claveLenta' lento.pprof
go test -race . && go test -bench . -benchmem -run xxx
```

## Resultados de referencia (Mac M1, 8 núcleos)

```
modo=lento  n=5000000 workers=8 tiempo=582ms
modo=rapido n=5000000 workers=8 tiempo=113ms        <- 5.1× más rápido, misma suma
```

**Perfil lento** (`Duration: 702ms, Total samples = 2800ms (398%)`), por *flat*:

```
 900ms 32.1%  runtime.usleep
 570ms 20.4%  runtime.madvise
 220ms  7.9%  runtime.pthread_cond_wait
 180ms  6.4%  runtime.pthread_kill
  50ms  1.8%  strings.ToUpper (170ms cum)
```

Más de la mitad del tiempo está en **funciones del runtime**, no en el código propio; en la vista `-cum` aparecen `runtime.lock2` / `osyield` (≈ 37 %): contención del asignador de memoria entre los 8 workers que asignan sin parar.

**`-list main.claveLenta`** (la línea exacta):

```
300ms   s := fmt.Sprintf("usuario-%d-%s", i, "region")
170ms   s = strings.ToUpper(s)
 40ms   h := sha256.Sum256([]byte(s))
230ms   return hex.EncodeToString(h[:])
```

**Perfil rápido:** el tiempo está en lo que debe estar: `hex.Encode` 44 % y SHA-256 (`blockSHA2`) 20 %; no hay funciones del runtime en las primeras posiciones.

**Benchmarks por llamada:**

```
BenchmarkClaveLenta    253.7 ns/op   184 B/op   5 allocs/op
BenchmarkClaveRapida    88.6 ns/op     0 B/op   0 allocs/op     <- 2.9×
```

## Conclusiones

- Con un solo hilo la mejora sería ~2.9× (benchmark); con 8 workers sube a **5.1×** porque además desaparece la contención del asignador: **reducir asignaciones escala mejor que reducir instrucciones**.
- El perfil por *flat* engaña cuando el coste real está en el runtime; la vista por *línea* (`-list`) señala el código a cambiar.
- Optimiza **solo lo que el perfil señale** y verifica con un benchmark y con el mismo resultado (la suma de comprobación y `TestLaVersionRapidaEsEquivalente`).

## Ejercicios propuestos

1. Abre el perfil en `go tool pprof -http=:8081` y mira el grafo y la vista *flame graph*.
2. Compara `-modo=lento` con `GOMAXPROCS=1`: ¿sigue apareciendo `runtime.lock2`?
3. Perfila con `-memprofile` (`go test -bench . -memprofile mem.out`) y usa `-alloc_space` para ver quién asigna.
4. Añade un `sync.Pool` a la versión lenta y compárala con la versión de búferes por worker.
