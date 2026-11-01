# Caso 6.3 — Ordenación por mezcla en paralelo  ·  `avanzado`

> **Problema:** Ordenar 4 millones de enteros usando todos los núcleos: ordenar las dos mitades en paralelo y mezclar, recursivamente.

## Qué features usa y por qué

| Feature de Go | Papel en este caso |
|--|--|
| Recursión + `go` | la mitad izquierda en otra goroutine, la derecha en la actual (no se desperdicia la goroutine que espera) |
| **Umbral de paralelismo** | por debajo de cierto tamaño no se lanzan goroutines |
| Caso base | ≤ 32 elementos: `slices.Sort` directo |
| Buffer auxiliar compartido por rangos | cada rama usa su trozo de `tmp`: sin solapes, sin cerrojos |
| `slices.Sort` | la referencia a batir (pdqsort, secuencial) |

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ Algoritmos divide y vencerás con subproblemas **independientes**.
- ✅ Datos grandes (millones de elementos).
- ❌ Datos pequeños: `slices.Sort` gana.
- ❌ La mezcla final es secuencial y limita el speedup (Amdahl): por eso ≈ 3.9× y no 8×.
- ❌ Sin umbral: millones de goroutines minúsculas.

## Cómo ejecutarlo

```bash
go run .
go test -race -v .
```

## Qué observar (salida real, Apple M1, Go 1.26.5)

```
ordenar 4000000 enteros, GOMAXPROCS=8 (mejor de 5)
  slices.Sort (secuencial, referencia)   287.98ms
  mezcla SECUENCIAL (propia, sin goroutines) 324.95ms
  mezcla paralela, umbral 8               113.41ms (2.9× vs mezcla secuencial)
  mezcla paralela, umbral 1024            86.97ms  (3.7×)
  mezcla paralela, umbral 16384           83.43ms  (3.9×)
  mezcla paralela, umbral 131072          84.99ms  (3.8×)
  mezcla paralela, umbral 1000000         105.30ms (3.1×)
```

## Errores típicos

- **Sin umbral**: la sobrecarga de goroutines minúsculas mata la ganancia (umbral 8: 2.9× frente a 3.9× con 16 384).
- Umbral demasiado alto: se desaprovechan núcleos (umbral 1 M: 3.1×).
- Compartir `tmp` entre ramas solapadas (carrera).
- Comparar contra la versión secuencial **propia** y no contra la de la biblioteca: `slices.Sort` es muy buena.

## Profundiza en el curso

- [merge sort paralelo con mezcla paralela](../../../13-rendimiento/07-merge-sort-paralelo)
- [la parte secuencial limita](../../../01-fundamentos/05-amdahl)
- [comparar con rigor estadístico](../../../13-rendimiento/05-benchmarks-y-benchstat)
- [ver cómo se reparte en el tiempo](../../../12-profiling/04-trace)

## Prueba tú

1. Haz también la **mezcla** en paralelo (ejercicio del módulo 13) y mide el cambio.
2. Calcula con Amdahl el speedup máximo si la mezcla final fuera el 25 % del tiempo.
3. Cambia `casoBase` a 4 y a 1000.
