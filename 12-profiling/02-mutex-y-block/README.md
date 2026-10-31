# Ejercicio 2: perfiles de contención (`mutex`) y de bloqueo (`block`)

## Teoría

El perfil de CPU no ve a las goroutines **esperando**. Dos perfiles adicionales sí, pero están **desactivados por defecto** (cuestan algo) y hay que activarlos:

| Perfil | Activar | Qué mide | Dónde atribuye la pila |
|--|--|--|--|
| **mutex** | `runtime.SetMutexProfileFraction(n)` | tiempo que **otras** goroutines esperaron por un mutex contendido (`sync.Mutex`/`RWMutex`) | a quien lo **libera** (`Unlock`): "esta sección crítica retrasó a los demás" |
| **block** | `runtime.SetBlockProfileRate(ns)` | tiempo que una goroutine estuvo **bloqueada** en una operación de sincronización (lock, channel, `WaitGroup`, `select`, `Cond`) | a la goroutine **bloqueada** |

- `SetMutexProfileFraction(1)` registra **todos** los eventos; `n` = en promedio 1 de cada `n`. `SetBlockProfileRate(1)` registra todos; un valor `ns` registra bloqueos de al menos `ns` nanosegundos (más los más largos, proporcionalmente).
- Con `1` el coste es alto: en producción usa fracciones del orden de 100 o 1000 (o actívalo temporalmente).
- Tipos de muestra: `delay` (tiempo acumulado), `contentions` (número de eventos). Cambia con `-sample_index=contentions`.
- Se pueden leer con `pprof.Lookup("mutex")` / `Lookup("block")` o en `/debug/pprof/mutex` y `/debug/pprof/block`.

### Cómo interpretarlos

- **mutex:** el *flat* suele caer en `sync.(*Mutex).Unlock`. El nombre útil está en la vista **acumulada** (`-cum`) o con `-list`: el último frame propio es la **sección crítica que bloquea a los demás**.
- **block:** muestra *dónde esperan* las goroutines. Cuidado con las **pistas falsas**: `sync.(*WaitGroup).Wait` de `main` y `chanrecv` de goroutines ociosas aparecen con mucho tiempo y **no son un problema** (esperan porque no tienen nada que hacer).

### Cómo se corrige la contención

1. **Reducir la sección crítica:** mover el trabajo *fuera* del lock (solo la actualización del estado dentro).
2. **Fragmentar** (sharding): N mutex, cada uno protege una parte; se elige por hash de la clave.
3. **Relleno** entre fragmentos (`_ [40]byte`) para evitar *false sharing* (dos locks en la misma línea de caché).
4. Cambiar la estructura: `atomic`, `sync.Map`, copia al escribir, o no compartir.

## Qué hace el programa

8 workers × 400 000 operaciones sobre un mapa, con tres escenarios:

- `global`: un `Mutex` para todo el mapa.
- `fragmentado`: 64 fragmentos, cada uno con su mutex.
- `canal`: un productor que se bloquea enviando a un consumidor lento (para ver el perfil *block* de un channel).

```bash
go run . -escenario=global      -mutex m.pprof -block b.pprof
go run . -escenario=fragmentado -mutex m.pprof -block b.pprof
go run . -escenario=canal       -block b.pprof
go tool pprof -top -cum m.pprof
go tool pprof -list 'AlmacenGlobal..Incrementar' m.pprof
go test -race .
```

## Resultados de referencia (8 núcleos)

```
escenario=global       tiempo=581ms
escenario=fragmentado  tiempo=130ms        <- 4.5× más rápido
escenario=canal        tiempo=71ms
```

**Perfil mutex, global** (`Total 1914 ms` de retraso, 175 709 contenciones):

```
 cum
 1838ms 96.0%  main.(*AlmacenGlobal).Incrementar      <- el responsable
 1617ms 84.5%  sync.(*Mutex).Unlock (flat)
  297ms 15.5%  runtime.unlock
```

**Perfil mutex, fragmentado:** `219 ms` de retraso en total (8.7× menos).

**Perfil block, global** (`2294 ms`):

```
1713ms 74.7%  sync.(*Mutex).Lock     <- esperas reales por el lock
 581ms 25.3%  sync.(*WaitGroup).Wait <- PISTA FALSA: main esperando a los workers
```

**Perfil block, canal:** `chansend1` 70 ms (el productor bloqueado esperando al consumidor lento) y `WaitGroup.Wait` 70 ms (main).

## Conclusiones

- Con 8 goroutines en un único lock, cada operación esperaba una media de ~4 ms acumulados: el 96 % del retraso proviene de **una** función. Fragmentar reduce la contención 8.7× y el tiempo total 4.5×.
- El perfil de **mutex** señala *quién retiene* el lock; el de **block** *quién espera*. Úsalos juntos.
- Un perfil `block` siempre contiene esperas legítimas (main esperando, workers ociosos): **filtra** (`-focus`, `-ignore`) antes de sacar conclusiones.
- Ambos perfiles tienen un coste: desactívalos o usa fracciones altas cuando termines de diagnosticar.

## Bug encontrado durante el desarrollo

La primera versión de `trabajo()` escribía en una variable global `sink` para que el compilador no eliminara el bucle. Con el mapa fragmentado (locks distintos) eso era una **carrera de datos real**, y `go test -race` la detectó. Se corrigió haciendo que `trabajo()` devuelva un valor que se usa dentro de la sección crítica. Moraleja: el código de benchmark también necesita `-race`.

## Ejercicios propuestos

1. Cambia `fragmentos` a 1, 4, 16 y 256 y grafica tiempo y retraso de mutex.
2. Quita el relleno `_ [40]byte` y mide si cambia algo (*false sharing*).
3. Sustituye `Mutex` por `RWMutex` en el almacén global y compara perfiles.
4. Activa `SetBlockProfileRate(10_000)` (10 µs) y observa qué eventos desaparecen.
