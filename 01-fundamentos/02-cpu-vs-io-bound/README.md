# Ejercicio 2: CPU-bound vs I/O-bound

## Teoría

- **CPU-bound:** el tiempo lo limita el procesador (cálculo, compresión, hashing, parseo). Más workers que núcleos **no** ayuda; solo añade cambios de contexto.
- **I/O-bound:** el tiempo se pierde esperando (red, disco, base de datos). Los workers pueden superar con creces el número de núcleos porque casi todos están bloqueados.

Regla práctica para dimensionar un worker pool:

| Carga | Workers recomendados |
|-------|----------------------|
| CPU-bound | ≈ `runtime.NumCPU()` |
| I/O-bound | muy superior a `NumCPU`, limitado por el recurso externo (conexiones, rate limit, memoria) |
| Mixta | medir |

Cuando una goroutine hace I/O de red, el runtime la aparca en el *netpoller* y libera el hilo para otra goroutine; por eso miles de goroutines esperando son baratas.

## Qué hace el programa

Procesa 64 tareas con un pool de `N` workers (channel + `WaitGroup.Go`) y mide el tiempo para cada tipo de carga.

```bash
go run .
```

## Resultado de referencia (8 núcleos)

```
 workers      CPU-bound      I/O-bound
       1         1.026s         3.254s
       2          556ms         1.639s
       4          324ms          814ms
       8          256ms          406ms      <- CPU deja de mejorar aquí
      16          243ms          203ms
      64          225ms           51ms      <- I/O sigue escalando
```

(La última fila aparece duplicada porque `8*NumCPU = 64` en una máquina de 8 núcleos.)

## Conclusiones

- CPU-bound: el tiempo se estabiliza al llegar a ~`NumCPU` workers.
- I/O-bound: el tiempo baja casi linealmente con los workers porque cada tarea es solo espera (64 tareas × 50 ms / 64 workers ≈ 50 ms).
- Más workers de lo necesario en CPU-bound no empeora mucho en Go (las goroutines son baratas), pero tampoco mejora.

## Ejercicios propuestos

1. Haz una tarea mixta (50 ms de CPU + 50 ms de sleep) y busca el número óptimo de workers.
2. Sustituye `Sleep` por una petición HTTP real a un servidor local con `httptest`.
3. ¿Qué pasa si la tarea I/O-bound está limitada a 10 conexiones simultáneas? Agrega un semáforo con un channel con buffer.
