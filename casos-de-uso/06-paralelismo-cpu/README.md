# Concepto 6 · Paralelismo: aprovechar todos los núcleos

> **Pregunta guía:** Tengo un cálculo pesado sobre muchos datos. ¿Cómo lo reparto entre los núcleos y cuándo NO compensa?

Hasta aquí la concurrencia servía para **solapar esperas**. El **paralelismo** es otra cosa: ejecutar cálculo a la vez en varios núcleos para terminar antes. La receta casi siempre es la misma —**particionar, calcular con estado local, combinar**— y la medición manda: hay límites (memoria, Amdahl) y overheads.

## Escalera de casos

| Nivel | Caso | Idea | Feature principal |
|--|--|--|--|
| 1 · básico | [1-basico-suma-paralela](./1-basico-suma-paralela) | sumar 10 M números en k trozos; cuándo no compensa | particionado + `WaitGroup` |
| 2 · intermedio | [2-intermedio-map-reduce](./2-intermedio-map-reduce) | contar palabras: mapa compartido (lento) frente a local + fusión | estado local, reduce |
| 3 · avanzado | [3-avanzado-merge-sort](./3-avanzado-merge-sort) | divide y vencerás con umbral; la parte secuencial limita | recursión + goroutines, umbral |

## Cómo elegir

| Si tu trabajo es… | Entonces… |
|--|--|
| **CPU puro** sobre datos divisibles | particiona en ≈ `GOMAXPROCS` trozos, estado local, combina (este concepto) |
| **E/S** (red, disco) | muchas más goroutines que núcleos (caso 7.1) |
| **memoria**-bound (recorrer arrays enormes) | el techo está en 2-4 hilos: más no ayuda (módulo 13) |
| tareas muy pequeñas | agrúpalas en **lotes**; el coste de coordinar puede superar al trabajo |
| con una parte secuencial grande | **Amdahl**: el speedup máximo es `1/(1−p)` |
| ya no cabe en una máquina | repartir en **procesos/contenedores** (caso 7.2) |

## Sigue con

Estos tres casos enseñan a *repartir*. Para **medir** si compensa (speedup, Amdahl, memoria) ve al [concepto 8](../08-medir-el-paralelismo); para **repartir mejor** (carga desigual, escaneo) al [concepto 9](../09-repartir-el-trabajo); y para cálculos que avanzan **por pasos** al [concepto 10](../10-simulaciones-por-fases).
