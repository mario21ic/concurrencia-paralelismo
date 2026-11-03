# Caso 9.3 — Suma de prefijos (escaneo) en tres fases  ·  `avanzado`

> **Problema:** `dst[i] = xs[0] ⊕ xs[1] ⊕ … ⊕ xs[i]`. Cada elemento depende del anterior: parece imposible repartirlo. ¿Cómo se paraleliza un cálculo con dependencias?

## Qué features usa y por qué

| Pieza | Papel |
|--|--|
| **Fase 1** (paralela) | cada worker hace el escaneo *local* de su bloque y guarda el total del bloque |
| **Barrera** (`wg.Wait`) | la fase 2 necesita los totales de **todos** los bloques |
| **Fase 2** (secuencial) | escaneo de los `k` totales (muy pocos): calcula el desplazamiento de cada bloque |
| **Fase 3** (paralela) | cada worker recorre su bloque otra vez y aplica su desplazamiento |
| **Genéricos** `[T any]` + `op func(a, b T) T` | el mismo algoritmo vale para sumas, máximos, productos de matrices… |
| **Asociatividad** | basta con que `⊕` sea asociativa; no hace falta que conmute. Por eso el desplazamiento se aplica **por la izquierda** (`op(desplaza[w], dst[i])`) |

Es el patrón «calcula localmente, combina los extremos, corrige localmente» que está detrás de filtrados paralelos, compactaciones, `radix sort` y analizadores léxicos paralelos.

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ Cuando tienes una operación asociativa sobre una secuencia larga y el cálculo por elemento es relevante.
- ✅ Como plantilla mental: si hay dependencias, pregúntate si puedes calcular **localmente** y **corregir** después.
- ❌ Datos pequeños o operación muy barata: el algoritmo paralelo hace **más trabajo** que el secuencial (≈ `1 + (k−1)/k` pasadas), y la memoria manda.
- ❌ Si la operación **no** es asociativa (p. ej. restas): el resultado cambiaría según el reparto.

## Cómo ejecutarlo

```bash
go run .
go test -race -v .
```

## Qué observar (salida real, Apple M1, Go 1.26.5)

```
GOMAXPROCS=8. Escaneo (suma de prefijos) con dos operaciones asociativas (mejor de 5).

suma de int64 (barata)      n = 1000000   secuencial 3.91ms    | k=2 2.86ms    (1.4×) | k=4 1.34ms    (2.9×) | k=8 1.01ms    (3.9×)
producto 2×2 mod p (cara)   n = 1000000   secuencial 8.25ms    | k=2 5.76ms    (1.4×) | k=4 3.16ms    (2.6×) | k=8 2.91ms    (2.8×)

suma de int64 (barata)      n = 8000000   secuencial 16.88ms   | k=2 14.25ms   (1.2×) | k=4 7.6ms     (2.2×) | k=8 6.62ms    (2.5×)
producto 2×2 mod p (cara)   n = 8000000   secuencial 65.56ms   | k=2 46.19ms   (1.4×) | k=4 25.92ms   (2.5×) | k=8 25.18ms   (2.6×)
```

Cómo leerlo:

- Con **2 workers solo se gana 1.2-1.4×**, no 2×: el algoritmo paralelo hace `1 + (k−1)/k` pasadas (la fase 3 no toca el primer bloque), es decir 1.5 pasadas con k=2. El techo ideal es `k / (1 + (k−1)/k)`: **1.33×** con 2 workers, **2.29×** con 4 y **4.27×** con 8.
- Con **4 workers** lo medido (2.2-2.9×) está en torno a ese techo. Con **8 workers** (2.5-3.9×) queda por debajo del 4.27× ideal en todas las filas, y con `n = 8 M` en solo 2.5-2.6×: ahí el límite ya no son los núcleos sino la memoria y la mezcla de núcleos P/E del M1 (ver [8.3](../../08-medir-el-paralelismo/3-avanzado-memoria-y-cache)).
- **Esperaba que la operación cara (producto de matrices) escalara claramente mejor que la suma barata, y los datos no lo confirman de forma consistente**: con `n = 8 M` salen casi iguales (2.5× frente a 2.6×), y con `n = 1 M` la suma incluso gana en esta ejecución (3.9× frente a 2.8×). En una ejecución anterior el secuencial de la suma con `n = 1 M` tardó 2.44 ms en vez de 3.91 ms: con trabajos de pocos milisegundos la variación entre ejecuciones es grande. Quédate con la forma —ganancia moderada, por debajo del techo ideal, lejos del 8× que sugeriría el número de núcleos— y no con las décimas.
- El test usa una operación **no conmutativa** (matrices) para detectar si se aplicara el desplazamiento por el lado equivocado, y comprueba que el escaneo de la matriz de Fibonacci produce los números de Fibonacci módulo `p`.

## Errores típicos

- Aplicar el desplazamiento por la derecha (`op(dst[i], desplaza[w])`): con operaciones no conmutativas da un resultado erróneo, y con la suma no se nota.
- Olvidar la barrera entre fases (leer `totales` mientras otro worker aún los escribe: carrera).
- Incluir la **reserva del slice** de salida dentro de lo que se mide: fallar páginas de cientos de MB puede dominar el tiempo y ocultar (o inventar) el speedup. Aquí `dst` se reserva fuera.
- Dar por hecho que «más pasadas en paralelo» siempre gana a «una pasada secuencial».

## Profundiza en el curso

- [la ley de Amdahl](../../../01-fundamentos/05-amdahl)
- [map-reduce](../../../13-rendimiento/06-map-reduce): otro esquema local + combinar
- [merge sort paralelo](../../../13-rendimiento/07-merge-sort-paralelo)
- [barreras y `sync.WaitGroup`](../../../02-goroutines/03-waitgroup)

## Prueba tú

1. Implementa el máximo acumulado (`max`) con `EscaneoParalelo`: ¿cambia el patrón de aceleración?
2. Cambia `k` a 64 con `n = 8 M`: ¿qué ocurre con la fase 2? ¿es realmente despreciable?
3. Usa el escaneo para **compactar** un slice (quedarte con los elementos que cumplen un predicado): calcula con un escaneo la posición de cada elemento en la salida.
