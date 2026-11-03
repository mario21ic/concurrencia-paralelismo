# Concepto 10 · Simulaciones por fases: paralelismo con barreras

> **Pregunta guía:** Mi cálculo avanza por pasos (generaciones, ticks, iteraciones) y cada paso depende del anterior. ¿Cómo lo paralelizo?

Hasta ahora el trabajo era una «bolsa» de tareas independientes. En las **simulaciones** no: la generación `t+1` necesita **toda** la `t`. No se puede repartir el *tiempo*, pero sí el *espacio* dentro de cada paso, con una **barrera** al final de cada fase para que nadie adelante a los demás. Es el esquema *bulk synchronous* de la computación científica, los motores de juegos y el entrenamiento de modelos.

## Escalera de casos

| Nivel | Caso | Idea | Técnica de sincronización |
|--|--|--|--|
| 1 · básico | [1-basico-montecarlo](./1-basico-montecarlo) | estimar π con millones de muestras independientes | ninguna: solo el generador de números aleatorios **por worker** |
| 2 · intermedio | [2-intermedio-juego-de-la-vida](./2-intermedio-juego-de-la-vida) | cada generación se reparte en bandas de filas | doble búfer + `WaitGroup` como barrera |
| 3 · avanzado | [3-avanzado-n-cuerpos](./3-avanzado-n-cuerpos) | N partículas, 2 fases por paso, workers que viven toda la simulación | barrera cíclica propia (`sync.Cond`) |

## Cómo elegir

| Si tu simulación… | Entonces… |
|--|--|
| son muestras independientes (Monte Carlo) | reparte las muestras y da a cada worker su **propio generador**; combina al final |
| avanza por pasos y cada celda lee a sus vecinas | **doble búfer** (leer de uno, escribir en otro) + barrera entre pasos |
| tiene varias fases por paso que dependen entre sí | una barrera **entre cada fase**; empieza por lanzar goroutines por fase (más simple) |
| los pasos son muy cortos (microsegundos) | agrupa más trabajo por paso o no paralelices: el coste de la barrera domina |
| los workers guardan estado propio caro (buffers, cachés) | workers persistentes con barrera cíclica |
