# Caso 9.1 — Multiplicar matrices repartiendo las filas  ·  `básico`

> **Problema:** Calcular `C = A × B` con matrices de 768×768 (≈ 0.9 GFLOP). Cada elemento de `C` se calcula sin depender de los demás. ¿Cómo lo reparto sin cerrojos?

## Qué features usa y por qué

| Feature | Papel en este caso |
|--|--|
| **Descomposición por la salida** | las filas de `C` se parten en `k` bandas contiguas; cada worker escribe SOLO en las suyas |
| Datos compartidos de **solo lectura** | `A` y `B` las leen todos los workers a la vez: sin escrituras no hay carrera ni hace falta cerrojo |
| `sync.WaitGroup.Go` | lanzar las bandas y esperar al final |
| Orden de bucles `i-k-j` | recorre `B` y `C` por filas (memoria contigua): mucho más amable con la caché que `i-j-k` |

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ Cada elemento de la salida depende solo de entradas **que no cambian** (matrices, imágenes, filtros, renderizado, simulaciones por celdas independientes).
- ✅ El coste por fila es parecido (si no, ver [9.2](../2-intermedio-carga-desigual)).
- ❌ Matrices pequeñas: coordinar `k` goroutines cuesta más que multiplicarlas.
- ❌ En producción usa una biblioteca de álgebra lineal optimizada (BLAS/gonum), que además hace *blocking* por caché y usa SIMD: este caso es didáctico.

## Cómo ejecutarlo

```bash
go run .
go test -race -v .
```

## Qué observar (salida real, Apple M1, Go 1.26.5)

```
GOMAXPROCS=8. C = A×B con matrices 768×768 (≈ 0.9 GFLOP), mejor de 3.

secuencial          231ms      1.00×   3.92 GFLOP/s
paralela k=2         115ms      2.02×   7.91 GFLOP/s
paralela k=4         59ms       3.88×   15.24 GFLOP/s
paralela k=8         51ms       4.53×   17.76 GFLOP/s
paralela k=16        51ms       4.57×   17.92 GFLOP/s
```

- Con 2 y 4 workers el escalado es casi lineal (**2.02×** y **3.88×**): el reparto por filas no necesita ninguna sincronización.
- Con 8 se llega a **4.53×**, no a 8×. El M1 combina 4 núcleos de rendimiento y 4 de eficiencia, así que 8 workers no son 8 núcleos iguales; además `A`, `B` y `C` suman ≈ 13.5 MiB, que probablemente presionan la caché y la memoria compartidas.
- 16 workers no mejoran a 8: más goroutines que núcleos no añade capacidad de cálculo.
- El test comprueba que el resultado paralelo es **idéntico bit a bit** al secuencial para cualquier `k`: cada elemento se calcula con las mismas operaciones en el mismo orden, solo cambia *quién* lo calcula.

## Errores típicos

- Repartir por **entradas** y acumular en la misma salida desde varios workers (carrera o mutex que serializa); reparte por **salidas**.
- Hacer que cada worker reciba una *copia* de `A` y `B` «para evitar compartir»: duplica memoria sin ganar nada, porque compartir lecturas es gratis.
- Olvidar el orden de bucles: el `i-j-k` ingenuo puede ser varias veces más lento y encima escala peor (la memoria manda).
- Dividir en tantas bandas que cada una sea diminuta.

## Profundiza en el curso

- [escalabilidad y sus techos](../../../13-rendimiento/01-escalabilidad)
- [map-reduce en profundidad](../../../13-rendimiento/06-map-reduce)
- [carreras de datos y cómo arreglarlas](../../../02-goroutines/07-arreglar-race)
- [el caso anterior de este concepto: suma paralela](../../06-paralelismo-cpu/1-basico-suma-paralela)

## Prueba tú

1. Cambia el orden de los bucles a `i-j-k` (acumulando `c[i][j] += a[i][k]*b[k][j]` con `j` en medio): ¿cuánto empeora el secuencial? ¿y la aceleración?
2. Reparte por **columnas** de `C` en vez de por filas: ¿se mantiene la corrección? ¿y el rendimiento?
3. Usa `n = 128` y `n = 2048`: ¿en cuál compensa menos paralelizar?
