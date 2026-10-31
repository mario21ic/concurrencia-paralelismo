# Ejercicio 3: contador fragmentado y estructuras por goroutine

## Teoría

Un contador compartido con muchas escrituras tiene un problema físico: **todos los núcleos pelean por la misma línea de caché** (ver ejercicio 2). Un atómico evita el lock, pero no esa pelea.

### Cuatro estrategias, de peor a mejor escalabilidad

| Estrategia | Escribir | Leer | Idea |
|--|--|--|--|
| **Un atómico** | contiende | O(1) | simple; no escala con muchas escrituras |
| **Mutex** | contiende y puede aparcar goroutines | O(1) | peor aún bajo contención |
| **Fragmentado** (N atómicos con relleno) | casi sin contención | **O(N)** | cada escritor toca un fragmento distinto; se suma al leer |
| **Acumulación local** | **sin sincronización** | — | cada goroutine cuenta en una variable propia y combina al final |

### Qué fragmento usa cada escritor

- **Aleatorio** (`rand.Uint32()`): fácil y portable. Pero cada línea la toca un núcleo distinto cada vez: **las líneas migran** entre núcleos (coste de coherencia) aunque casi no haya colisiones.
- **Afín** (índice estable por goroutine/worker): cada línea la usa casi siempre el mismo núcleo ⇒ **no migra**. Es lo más cercano a un contador "por procesador" (Go no expone el número de `P`; `sync.Pool` y otros paquetes internos usan `runtime_procPin`).

### El compromiso: leer es caro

El total es la **suma** de los fragmentos: O(N) lecturas y solo es "exacto" si no hay escrituras en curso (una suma de lecturas no simultáneas; nunca retrocede). Es ideal para **métricas** (muchas escrituras, lecturas ocasionales) y mala si se lee en cada operación.

### Cuándo usarlo

Contadores de peticiones, bytes, errores, histogramas de latencia: alta frecuencia de escritura desde muchos hilos y lectura periódica. Para un contador que se escribe poco, un `atomic.Int64` basta.

## Qué contiene

- `contador.go`: `Atomico`, `ConMutex`, `FragmentadoSinRelleno`, `Fragmentado` (relleno con `cpu.CacheLinePad`; `Sumar` aleatorio y `SumarEn` afín).
- `contador_test.go`: suma exacta con 16 goroutines, fragmentos en líneas propias, `Total()` monótono con escrituras concurrentes (con `-race`).
- `contador_bench_test.go`: benchmarks con `RunParallel`.

```bash
go test -race .
go test -bench 'Atomico$|Mutex$|Fragmentado|Acumulacion' -run xxx -cpu 1,2,4,8
go test -bench Lectura -run xxx
```

## Resultados de referencia (ns por operación; menos es mejor; `-cpu` = nº de goroutines en paralelo)

`RunParallel` reporta tiempo de reloj por operación del conjunto: con escalado **ideal**, `ns/op` se divide por el nº de CPUs.

| Variante | 1 CPU | 2 CPUs | 4 CPUs | 8 CPUs | Comentario |
|--|--|--|--|--|--|
| Un atómico | 7.5 | 17.6 | 26.9 | **34.6** | **empeora** 4.6× al añadir CPUs |
| Mutex | 15.4 | 41.9 | 72.8 | **103.2** | empeora 6.7× |
| Fragmentado sin relleno (aleatorio) | 7.5 | 14.2 | 14.9 | 23.0 | línea compartida: false sharing |
| Fragmentado con relleno (aleatorio) | 7.7-8.4 | 10.1 | 9.6-11.9 | 9.5-12.5 | plano: **no escala** |
| **Fragmentado afín** | 8.1 | 3.8 | 2.1 | **1.46** | **escala** (5.6× a 8 CPUs) |
| Acumulación local | 0.70 | 0.34 | 0.24 | **0.25** | casi gratis |

**Coste de leer el total:** atómico 2.3 ns; fragmentado (64 fragmentos) **48.9 ns** (21× más).

## Conclusiones

- **Escalado negativo:** con un atómico, añadir núcleos **empeora** cada operación (de 7.5 a 34.6 ns). El mutex es peor (103 ns).
- **El relleno importa:** sin él el fragmentado queda en 23 ns; con él en ~10 ns.
- **Pero fragmentar al azar no basta**: se queda plano (~10 ns) en vez de bajar a ~1 ns. Se comprobó la hipótesis de la migración de líneas con la variante **afín**: la misma estructura, solo con un índice estable por worker, baja a **1.46 ns** (la diferencia con el aleatorio es de ~7× con 8 CPUs). Primera versión del ejercicio: el fragmentado aleatorio "parecía" la solución; solo medir con `-cpu 1,2,4,8` mostró que no escalaba.
- **La mejor sincronización es la que no existe:** la acumulación local es 140× más rápida que el atómico (0.25 vs 34.6 ns) a 8 CPUs, a costa de combinar al final.
- **El precio:** leer cuesta 21× más.

## Ejercicios propuestos

1. Cambia `fragmentos` a 8, 16 y 256 y mide: ¿cuánto hace falta para 8 CPUs?
2. Implementa un histograma de latencias fragmentado (un array de cubos por fragmento).
3. Compara con `sync/atomic` de 32 bits y con `atomic.Uint64`.
4. Busca cómo `sync.Pool` o `expvar` evitan el false sharing por dentro.
