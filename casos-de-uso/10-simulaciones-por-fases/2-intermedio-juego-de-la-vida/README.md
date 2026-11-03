# Caso 10.2 — Juego de la Vida: paralelismo por generaciones  ·  `intermedio`

> **Problema:** En el Juego de la Vida de Conway, la generación `t+1` depende de **toda** la generación `t`. No se puede repartir el tiempo, pero sí el espacio dentro de cada generación. ¿Cómo lo hago sin errores y cuándo compensa?

## Qué features usa y por qué

| Pieza | Papel |
|--|--|
| **Bandas de filas** | cada worker calcula las filas `[w·n/k, (w+1)·n/k)` de la siguiente generación |
| **Doble búfer** | se lee de `act` y se escribe en `sig`; al acabar la generación se intercambian. Si se escribiera sobre el mismo tablero, los vecinos leerían celdas ya actualizadas |
| **Barrera** | `wg.Wait()` al final de cada generación: nadie empieza la `t+1` hasta que acaban todas las bandas de la `t` |
| `sync.WaitGroup.Go` | crear las goroutines de cada generación |
| Malla toroidal | los bordes se unen: todas las celdas tienen 8 vecinas, sin casos especiales |

Cada worker **lee** filas ajenas (las vecinas del borde de su banda) pero **escribe** solo las suyas: lecturas compartidas sin carreras, porque `act` no se modifica durante la generación.

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ Mallas, autómatas, difusión de calor, simulación de fluidos, procesamiento de imágenes iterativo: celdas que dependen de sus vecinas en la generación anterior.
- ✅ Tableros grandes: el cálculo por generación supera con holgura el coste de coordinar.
- ❌ **Tableros pequeños o pocas celdas por generación**: lanzar y esperar `k` goroutines en cada paso cuesta microsegundos, que pueden superar al cálculo (ver la primera fila de la tabla).
- ❌ Si las celdas se actualizan «en el sitio» (Gauss-Seidel) hay dependencias dentro de la misma generación y este esquema no vale tal cual.

## Cómo ejecutarlo

```bash
go run .
go test -race -v .
```

## Qué observar (salida real, Apple M1, Go 1.26.5)

```
GOMAXPROCS=8. Juego de la Vida toroidal, mejor de 3. Aceleración frente a k=1.

tablero            generaciones k=1          k=2              k=4              k=8             
32×32              20000        41.3ms       64.7ms (0.6×)    64ms (0.6×)      71.2ms (0.6×)   
128×128            5000         202.6ms      180.5ms (1.1×)   160.8ms (1.3×)   142.3ms (1.4×)  
512×512            500          526.7ms      281.4ms (1.9×)   175.9ms (3.0×)   166ms (3.2×)    
2048×2048          40           1.1295s      576.7ms (2.0×)   307.4ms (3.7×)   272.8ms (4.1×)  
```

- La misma simulación pasa de **0.6× (más lenta en paralelo)** con 32×32 a **4.1×** con 2048×2048. Lo que cambia es la **granularidad**: con 32×32 una generación cuesta ≈ 2 µs en secuencial, menos que lanzar y esperar 8 goroutines; con 2048×2048 una generación son ≈ 28 ms.
- En tableros medianos (128×128) la ganancia es mínima (1.4×): hay cálculo, pero poco para repartir en 8.
- Los tests comprueban que el resultado paralelo es **idéntico** al secuencial (tableros aleatorios, varios `k`, incluso `k` mayor que las filas), que un planeador se desplaza (+1,+1) cada 4 generaciones en el toro y que `Evoluciona` no modifica el tablero inicial.

## Errores típicos

- Escribir en el mismo tablero del que se lee: resultado incorrecto **y** carrera.
- Olvidar la barrera, o intercambiar los tableros antes de que todas las bandas hayan terminado.
- Paralelizar un tablero pequeño «porque son goroutines baratas»: las goroutines son baratas, pero no gratis.
- Repartir por columnas en una malla guardada por filas: cada worker salta por memoria y pierde localidad de caché.

## Profundiza en el curso

- [WaitGroup como barrera](../../../02-goroutines/03-waitgroup)
- [escalabilidad y sus techos](../../../13-rendimiento/01-escalabilidad)
- [lotes: amortizar el coste de coordinar](../../../13-rendimiento/04-batching)
- [matrices por filas](../../09-repartir-el-trabajo/1-basico-matrices): mismo reparto por la salida
- [siguiente caso: barrera cíclica con workers persistentes](../3-avanzado-n-cuerpos)

## Prueba tú

1. Busca el tamaño de tablero a partir del cual `k=8` empieza a ganar al secuencial.
2. Con 512×512, prueba `k=16` y `k=32`: ¿mejora?
3. Cuenta las celdas vivas tras 100 generaciones con `Vivas()` para distintas semillas: ¿se estabiliza la población?
