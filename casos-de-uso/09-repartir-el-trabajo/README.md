# Concepto 9 · Repartir el trabajo: cómo se divide importa

> **Pregunta guía:** Voy a repartir un cálculo entre workers. ¿Cómo lo divido para que todos terminen a la vez y no se pisen?

Paralelizar es **descomponer**: partir el trabajo en trozos que puedan ejecutarse a la vez. La forma de partirlo decide si escala o no. Hay tres preguntas, una por caso:

1. ¿Sobre qué divido? **Sobre la salida** (cada worker escribe un rango propio) y no habrá carreras → [9.1](./1-basico-matrices).
2. ¿Cuestan todos los trozos lo mismo? Si no, un reparto fijo deja a unos esperando → [9.2](./2-intermedio-carga-desigual).
3. ¿Y si cada elemento depende del anterior? A veces aún se puede, con un algoritmo de varias fases → [9.3](./3-avanzado-prefix-sum).

## Escalera de casos

| Nivel | Caso | Idea | Técnica |
|--|--|--|--|
| 1 · básico | [1-basico-matrices](./1-basico-matrices) | multiplicar matrices 768×768 repartiendo las filas de la salida | descomposición de dominio, sin cerrojos |
| 2 · intermedio | [2-intermedio-carga-desigual](./2-intermedio-carga-desigual) | Mandelbrot: estático, cíclico y dinámico (contador atómico) | equilibrio de carga |
| 3 · avanzado | [3-avanzado-prefix-sum](./3-avanzado-prefix-sum) | suma de prefijos (escaneo) con 3 fases y operación asociativa | algoritmo paralelo con barreras |

## Cómo elegir el reparto

| Si el trabajo es… | Reparte… | Por qué |
|--|--|--|
| de coste **igual** por elemento y datos contiguos | **estático**, en `k` bloques contiguos | sin coordinación, buena localidad de caché |
| de coste **desigual pero repartido** (los caros están entremezclados) | **cíclico** (`i % k`) | reparte las zonas caras sin coordinar |
| de coste **desconocido o muy irregular** | **dinámico** con contador atómico o `chan`, en lotes | el que termina antes coge más; el lote reduce el tráfico |
| con dependencias entre elementos | un **algoritmo de fases** (local → combinar → corregir) | 9.3 |
| con muchas tareas diminutas | agrúpalas en **lotes** (el coste de coordinar no puede superar al de calcular) | 9.2 con lote de 16 |
