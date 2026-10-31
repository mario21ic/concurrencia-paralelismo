# Ejercicio 1: concurrencia vs paralelismo

## Teoría

| | Concurrencia | Paralelismo |
|--|--|--|
| Es... | una forma de **estructurar** el programa | una forma de **ejecutarlo** |
| Idea | lidiar con muchas cosas a la vez | hacer muchas cosas a la vez |
| Requiere | tareas independientes | varios núcleos |
| Un solo núcleo | sí (intercalado) | no |

*"Concurrency is not parallelism"* (Rob Pike): un diseño concurrente **permite** ejecutar en paralelo si el hardware lo ofrece, pero es útil por sí mismo.

En Go, `GOMAXPROCS` es el número de **P** (procesadores lógicos del scheduler), es decir, cuántas goroutines pueden ejecutar código Go simultáneamente. Con `GOMAXPROCS=1` tienes concurrencia pero **no** paralelismo.

## Qué hace el programa

Corre 8 tareas de tres maneras (secuencial, concurrente con `GOMAXPROCS=1`, concurrente con todos los núcleos) para dos tipos de tarea:

- **CPU-bound:** un bucle que suma.
- **I/O-bound:** `time.Sleep(200ms)` simulando una espera de red.

```bash
go run .
```

## Resultado de referencia (8 núcleos)

```
== CPU-bound ==
secuencial:                    771ms
concurrente, GOMAXPROCS=1:     767ms     <- sin mejora
concurrente, GOMAXPROCS=8:     147ms     <- ~5x

== I/O-bound ==
secuencial:                    1.606s
concurrente, GOMAXPROCS=1:     201ms     <- ~8x con UN solo núcleo
concurrente, GOMAXPROCS=8:     201ms
```

## Conclusiones

1. **CPU-bound + 1 P:** concurrente ≈ secuencial. Hay intercalado, pero el único núcleo siempre está ocupado, así que no se gana nada.
2. **CPU-bound + 8 P:** el paralelismo real reduce el tiempo (no exactamente 8x por overhead y otros procesos del SO).
3. **I/O-bound:** la concurrencia sola basta. Mientras una goroutine espera, el scheduler ejecuta otra; no hace falta paralelismo.

## Ejercicios propuestos

1. Cambia `tareas` a 2 y 16. ¿Cómo cambia el speedup con 8 núcleos?
2. Usa `GOMAXPROCS=2` y `GOMAXPROCS=4` y grafica tiempo vs núcleos.
3. Reemplaza `wg.Go(f)` por la forma clásica (`wg.Add(1)` + `go func(){ defer wg.Done(); f() }()`).
