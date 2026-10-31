# Ejercicio 5: Ley de Amdahl

## Teoría

Si una fracción **p** del programa es paralelizable y **(1 − p)** es serial, el speedup máximo con **N** núcleos es:

```
speedup(N) = 1 / ((1 − p) + p / N)
```

Con N → ∞: `speedup_max = 1 / (1 − p)`.

| p paralelizable | Speedup máximo |
|--|--|
| 50 % | 2x |
| 90 % | 10x |
| 95 % | 20x |
| 99 % | 100x |

Implicaciones:

- La parte **serial** domina: reducir un 5 % de serialización vale más que duplicar núcleos.
- En Go, secciones seriales típicas son: un `Mutex` muy disputado, un único productor, un channel cuello de botella, el *merge* final.
- Amdahl asume trabajo de tamaño fijo. La **ley de Gustafson** dice que, si el problema crece con los núcleos, el speedup escala mejor.
- En la práctica además hay overhead de coordinación, que Amdahl no modela.

## Qué hace el programa

1. Imprime la tabla teórica para varios `p` y `N`.
2. Mide un programa real: cada tarea hace trabajo paralelo y luego una sección serial protegida por un mutex. Compara el speedup medido con el de Amdahl.

```bash
go run .
```

## Resultado de referencia

```
p \ N          1        2        4        8       16       64  1048576
0.90        1.00     1.82     3.08     4.71     6.40     8.77    10.00
0.99        1.00     1.98     3.88     7.48    13.91    39.26    99.99

tarea ≈ 12ms, parte serial ≈ 2ms, p ≈ 0.83
 workers       tiempo    speedup     Amdahl
       1        751ms      1.01x      1.00x
       2        388ms      1.96x      1.71x
       4        219ms      3.47x      2.67x
       8        161ms      4.70x      3.69x
```

## Cómo leer el resultado (importante)

El speedup medido **supera** al teórico. No viola Amdahl; es un fallo de mi estimación de `p`:

- `p` se calcula con una sola medición de la tarea, que en frío es más lenta de lo normal (calentamiento de CPU/caché), así que `p` sale algo subestimado.
- Además, en este diseño la sección serial de un worker se solapa con el trabajo paralelo de los demás, por lo que el modelo "serial puro" es pesimista.

Lo que sí se observa con claridad es la **tendencia**: el speedup crece cada vez menos al añadir núcleos (de 4 a 8 workers ganas ~1.2x en vez de 2x) porque la sección serial se vuelve el cuello de botella.

## Ejercicios propuestos

1. Aumenta `serial` (por ejemplo `unidad/2`) y mira cómo se aplana la curva.
2. Calcula `p` promediando varias mediciones tras un calentamiento y compara de nuevo.
3. Reduce la sección serial sacando `busy` fuera del lock y acumulando resultados locales por worker; mide la mejora.
