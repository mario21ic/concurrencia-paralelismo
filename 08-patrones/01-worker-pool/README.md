# Ejercicio 1: worker pool

## Teoría

Un **worker pool** ejecuta N tareas con **como máximo W goroutines** (W ≪ N). Los workers toman trabajo de un channel compartido; el sistema nunca tiene más de W tareas en vuelo.

```
        ┌─► worker 1 ─┐
trabajos ├─► worker 2 ─┼─► resultados
        └─► worker W ─┘
```

Por qué no "una goroutine por tarea": con N grande (millones de filas, miles de archivos) se agotan descriptores, conexiones, memoria o el límite de la API remota. El pool convierte "N" en un parámetro acotado.

### Diseño de `Procesar[I, O]`

- **Entrada y salida genéricas**; `f` recibe el `ctx`.
- El channel `trabajos` transporta **índices**, no valores: cada worker escribe en `res[i]`. Así el resultado **conserva el orden de entrada** y no hace falta mutex (posiciones distintas; `wg.Wait()` da el happens-before para leer).
- **Cancelación en tres sitios:**
  1. el alimentador usa `select` con `ctx.Done()` y marca como cancelado lo no enviado;
  2. el worker comprueba `ctx.Err()` antes de empezar cada tarea;
  3. `f` recibe `ctx` para abortar internamente.
- El número de workers se limita a `len(entradas)` (no lanzar más de los necesarios).
- Los errores son **por elemento** (`Resultado.Err`): un fallo no aborta a los demás. Para "cancelar todo al primer error" se usa `errgroup` (módulo 10).

### Elegir W

I/O-bound: tantos como admita el recurso externo. CPU-bound: ≈ `NumCPU`. En la duda, que sea configurable y medir.

## Qué hace el programa

1. 64 tareas de 20 ms con 1, 2, 4, 8, 16 y 64 workers; mide el tiempo y la concurrencia máxima observada.
2. Demuestra que el orden se conserva aunque las tareas terminen desordenadas.
3. Plazo de 100 ms: cuántas tareas terminan, fallan por sí mismas o se cancelan.

```bash
go run .
go test -race -count=3 .
```

## Resultado de referencia

```
 workers       tiempo máx. simultáneas
       1       1.337s                1
       2        667ms                2
       4        334ms                4
       8        168ms                8
      16         84ms               16
      64         21ms               64

resultados en orden de entrada: r5 r1 r4 r2 r3

con plazo de 100ms: 18 ok, 2 con error propio, 44 cancelados
```

## Conclusiones

- El tiempo escala **linealmente** con W (tarea I/O-bound), y la concurrencia máxima observada es exactamente W: el límite se cumple.
- El orden de los resultados es el de la entrada aunque la ejecución sea desordenada.
- Con plazo: 18 + 2 + 44 = 64. Ninguna tarea se pierde ni se queda sin estado.

## Tests (`pool_test.go`)

Orden y límite de concurrencia, cancelación a mitad, ausencia de fugas de goroutines, entrada vacía.

## Ejercicios propuestos

1. Cambia la tarea a CPU-bound (bucle de cálculo) y repite la tabla: ¿dónde se detiene la mejora?
2. Añade reintentos con *backoff* por tarea.
3. Haz que `Procesar` devuelva también un `[]error` agregado con `errors.Join`.
4. Implementa una variante con **pool persistente** (workers de larga vida y `Enviar(tarea)`) y su `Cerrar()` ordenado.
