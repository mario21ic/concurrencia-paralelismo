# Caso 8.2 — La ley de Amdahl, medida  ·  `intermedio`

> **Problema:** Mi tarea tiene una parte que se puede repartir y otra que no (leer la entrada, fusionar resultados, escribir la salida…). ¿Cuánto puedo esperar ganar con más núcleos?

## Qué features usa y por qué

| Feature | Papel en este caso |
|--|--|
| Fracción serial `s` | parte del trabajo que se ejecuta en un solo hilo antes del reparto |
| Reparto estático y equilibrado | 64 bloques idénticos asignados `b = w, w+k, …`: así la parte paralela escala bien y lo único que frena es la serial |
| `sync.WaitGroup.Go` | lanzar los workers de la parte paralela |
| Fórmula de Amdahl | `speedup(k) ≤ 1 / ( s + (1−s)/k )`; con infinitos núcleos, `≤ 1/s` |

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ Antes de invertir en paralelizar: **calcula `s`** (¿qué fracción del tiempo es inevitablemente secuencial?) y tendrás el techo.
- ✅ Para decidir *dónde* optimizar: si `s = 25 %`, ningún número de núcleos pasará de 4×; hay que reducir `s`.
- ❌ La ley supone trabajo total fijo. Si al tener más núcleos *aumentas el problema* (más datos), importa más la ley de Gustafson: el techo se aleja.
- ❌ Ignora costes de coordinación, memoria y heterogeneidad del hardware: es un **techo**, no una predicción.

## Cómo ejecutarlo

```bash
go run .
go test -race -v .
```

## Qué observar (salida real, Apple M1, Go 1.26.5)

```
GOMAXPROCS=8. Tarea de 400 M iteraciones; fracción serial s (mejor de 3).
Cada celda: aceleración medida (teórica de Amdahl).

s serial   T(1)       k=2             k=4             k=8             techo 1/s
0 %        779ms      1.96× (2.00×)   3.73× (4.00×)   5.00× (8.00×)   ∞
5 %        774ms      1.77× (1.90×)   3.15× (3.48×)   3.30× (5.93×)   20×
25 %       777ms      1.49× (1.60×)   2.10× (2.29×)   1.94× (2.91×)   4×
50 %       850ms      1.39× (1.33×)   1.65× (1.60×)   1.78× (1.78×)   2×
```

Cómo leerlo:

- **Cuanto mayor es `s`, antes se aplana la curva.** Con `s = 50 %` solo se ganó 1.78× con 8 workers; con `s = 25 %`, 8 workers (1.94×) rindieron igual o peor que 4 (2.10×).
- Con `s = 5 %` (parece poco) el techo teórico con 8 núcleos es 5.93×, y con infinitos núcleos 20×. Un 5 % serial ya baja el máximo teórico con 8 núcleos de 8× a 5.93×.
- **Lo medido queda por debajo de lo teórico** casi siempre. La fórmula supone 8 núcleos idénticos; la fila `s = 0 %` es el techo *real* de esta máquina (5.00× con 8 workers, por sus núcleos P/E y la carga de fondo). Lo que cuenta es la **forma**: cae con `s` y coincide con la teoría cuando el serial domina (`s = 50 %`: 1.78× medido y 1.78× teórico).
- Los 400 M de iteraciones se reparten igual en todas las filas; la fila de `s = 50 %` tiene `T(1)` algo mayor (850 ms frente a ≈ 777 ms) por variación de la ejecución.

## Errores típicos

- Paralelizar un bucle y olvidar que **leer la entrada, ordenar o fusionar** al final sigue siendo secuencial.
- Proteger la sección más caliente con un `Mutex`: ahí `s ≈ 100 %` para esa sección (ver [6.2](../../06-paralelismo-cpu/2-intermedio-map-reduce): el mutex global fue 9.8× *más lento* que el secuencial).
- Medir el speedup solo con el máximo de workers; la curva completa (2, 4, 8) revela dónde se aplana.
- Culpar al `go` runtime cuando la causa es el algoritmo.

## Profundiza en el curso

- [la ley de Amdahl, con ejercicios](../../../01-fundamentos/05-amdahl)
- [escalabilidad y sus techos](../../../13-rendimiento/01-escalabilidad)
- [map-reduce: reducir la parte serial al mínimo](../../../13-rendimiento/06-map-reduce)
- [merge sort paralelo: la fusión como parte serial](../../../13-rendimiento/07-merge-sort-paralelo)

## Prueba tú

1. Añade `s = 10 %` y `s = 90 %`: ¿se cumple la forma de la curva?
2. Estima `s` de uno de tus programas: mide cuánto tarda con `GOMAXPROCS=1` la parte que *no* se paraleliza, dividida por el total.
3. Sustituye el reparto estático por uno desigual (la mitad de los bloques el doble de grandes): ¿qué se añade a Amdahl? (ver [9.2](../../09-repartir-el-trabajo/2-intermedio-carga-desigual))
