# Caso 9.2 — Carga desigual: estático, cíclico o dinámico  ·  `intermedio`

> **Problema:** Pintar el conjunto de Mandelbrot (1024×1024). Cada píxel cuesta entre 1 y 800 iteraciones: las filas del centro cuestan mucho más que las de los bordes. Si doy un bloque fijo de filas a cada worker, ¿qué pasa?

## Qué features usa y por qué

| Estrategia | Cómo reparte las filas | Coordinación |
|--|--|--|
| **Estática** | bloques contiguos de `n/k` filas | ninguna |
| **Cíclica** | la fila `y` va al worker `y % k` | ninguna |
| **Dinámica (1)** | un `atomic.Int64` entrega la siguiente fila libre | un `Add` por fila |
| **Dinámica (16)** | el mismo contador, entregando lotes de 16 filas | un `Add` cada 16 filas |

Métrica de equilibrio: `máx / media` del coste (iteraciones) que asumió cada worker; **1.00 = reparto perfecto**, 2.68 = el más cargado hizo 2.7× la media.

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ **Estática**: coste uniforme por elemento. Es la más simple y con mejor localidad.
- ✅ **Cíclica**: coste variable pero sin zonas enormes contiguas; no necesita coordinación.
- ✅ **Dinámica con lotes**: coste imprevisible (peticiones, ficheros, búsquedas, partidas…). Es lo que hace un *worker pool* con un `chan`.
- ❌ Dinámica de uno en uno cuando cada unidad es muy barata: el contador compartido se convierte en el cuello de botella; sube el tamaño del lote.
- ❌ Cíclica si el coste sigue un patrón periódico que coincide con `k` (los caros siempre caen al mismo worker).

## Cómo ejecutarlo

```bash
go run .
go test -race -v .     # tarda unos segundos: varios Mandelbrot completos
```

## Qué observar (salida real, Apple M1, Go 1.26.5)

```
GOMAXPROCS=8, k=8 workers. Mandelbrot 1024×1024 (mejor de 3).

secuencial       434ms      1.00×

estrategia       tiempo     aceleración desequilibrio (máx/media)
estático         154ms      2.81×       2.68
cíclico          87ms       4.98×       1.01
dinámico (1)     84ms       5.19×       1.10
dinámico (16)    82ms       5.32×       1.19
```

- **El mismo número de workers, casi el doble de tiempo.** El reparto estático tarda 154 ms (2.81×) frente a ≈ 85 ms de las otras (≈ 5×). No es un problema de cuántos hilos hay, sino de **uno** que carga con la mitad del trabajo (desequilibrio **2.68**: el worker más cargado hizo 2.7× la media, y los demás esperaron).
- La **cíclica** consigue un desequilibrio de **1.01** sin coordinar nada: las filas caras quedan repartidas entre todos.
- Las **dinámicas** quedan en 1.10 y 1.19 (el último lote puede ser desigual) y son igual de rápidas, con la ventaja de que **no hace falta conocer cómo se distribuye el coste**. Con lote 16 se hacen 16× menos operaciones sobre el contador atómico y el tiempo no empeora (82 ms).
- Todas dan la **misma imagen** (el test lo comprueba para varios `k`); y un segundo test verifica con el coste en iteraciones —no con el reloj— que el estático está más desequilibrado que el dinámico.

## Errores típicos

- Repartir bloques fijos y asumir que «todos tardarán lo mismo» sin medirlo.
- Medir solo el tiempo total y no el **tiempo por worker**: la causa del retraso queda oculta (aquí se mide el coste por worker).
- Usar un `chan` sin buffer para entregar unidades diminutas: el coste de pasar el valor puede superar al del trabajo.
- Que el reparto dinámico entregue el trabajo caro al final: el último lote caro se ejecuta solo y alarga la cola. Entrega primero lo más caro si lo conoces.

## Profundiza en el curso

- [worker pool (reparto dinámico con channels)](../../05-limitar-recursos/2-intermedio-worker-pool)
- [escalabilidad y sus techos](../../../13-rendimiento/01-escalabilidad)
- [batching: agrupar para amortizar el coste fijo](../../../13-rendimiento/04-batching)
- [la ley de Amdahl](../../../01-fundamentos/05-amdahl): un worker rezagado es una parte «serial» involuntaria

## Prueba tú

1. Prueba lotes de 4, 64 y 256 en `Dinamica`: ¿cuándo empieza a empeorar el equilibrio?
2. Cambia el rango de la imagen para que el conjunto quede arriba (no en el centro): ¿sigue siendo el estático tan malo? ¿y el cíclico?
3. Implementa la estrategia dinámica con un `chan int` de filas en vez del contador atómico y compara tiempos.
