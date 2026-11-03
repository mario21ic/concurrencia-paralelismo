# Concepto 8 · Medir el paralelismo: ¿cuánto acelera de verdad?

> **Pregunta guía:** He repartido mi cálculo entre varios núcleos. ¿Cuánto más rápido va, cuánto *debería* ir, y qué me lo impide?

El concepto 6 enseña a *repartir* un cálculo. Este enseña a **medirlo y explicarlo**: sin las tres ideas de aquí abajo es fácil creerse que «8 goroutines = 8× más rápido» y no entender por qué no pasa.

## Escalera de casos

| Nivel | Caso | Idea | Qué aprendes a medir |
|--|--|--|--|
| 1 · básico | [1-basico-speedup](./1-basico-speedup) | 256 bloques de CPU pura con 1…32 workers, y las mismas goroutines con `GOMAXPROCS=1` | aceleración, eficiencia, y la diferencia entre **concurrencia y paralelismo** |
| 2 · intermedio | [2-intermedio-amdahl](./2-intermedio-amdahl) | tareas con 0 %, 5 %, 25 % y 50 % de trabajo serial | la **ley de Amdahl**: lo medido frente a lo predicho |
| 3 · avanzado | [3-avanzado-memoria-y-cache](./3-avanzado-memoria-y-cache) | tres trabajos «paralelos» con curvas de escalado muy distintas | **false sharing** y **ancho de banda de memoria** como techo |

## Cómo elegir (diagnóstico de un paralelismo que no escala)

| Síntoma | Causa probable | Dónde se ve |
|--|--|--|
| el speedup se estanca en ≈ nº de núcleos | normal: no hay más hardware | [8.1](./1-basico-speedup) |
| el speedup se estanca *muy por debajo* de los núcleos | **fracción serial** (Amdahl), o un cerrojo que serializa | [8.2](./2-intermedio-amdahl) |
| 8 workers apenas mejoran a 4 y el trabajo apenas calcula | límite de **memoria** | [8.3](./3-avanzado-memoria-y-cache) (kernel C) |
| los workers «no se tocan» y aun así van lentos | **false sharing** | [8.3](./3-avanzado-memoria-y-cache) (kernel B) |
| más lento que el secuencial | trabajo demasiado pequeño para el coste de coordinar, o un mutex global | [6.1](../06-paralelismo-cpu/1-basico-suma-paralela), [6.2](../06-paralelismo-cpu/2-intermedio-map-reduce) |
| un worker termina mucho después que los demás | reparto desigual | [9.2](../09-repartir-el-trabajo/2-intermedio-carga-desigual) |
