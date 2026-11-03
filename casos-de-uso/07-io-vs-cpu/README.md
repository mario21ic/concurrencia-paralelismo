# Concepto 7 · Elegir el grado de paralelismo (E/S frente a CPU) y salir del proceso

> **Pregunta guía:** ¿Cuántas goroutines pongo? ¿Y cuándo hay que pasar de una máquina a varias?

Esta es la decisión práctica que cierra el bloque de paralelismo: el número de workers depende de **en qué se gasta el tiempo**, y cuando una máquina no basta, el siguiente escalón es repartir el trabajo entre **procesos o contenedores**.

## Escalera de casos

| Nivel | Caso | Idea | Feature principal |
|--|--|--|--|
| 1 · básico | [1-basico-cuantos-workers](./1-basico-cuantos-workers) | 256 tareas de E/S y de CPU con 1…256 workers | pool de workers |
| 2 · avanzado | [2-avanzado-contenedores](./2-avanzado-contenedores) | por qué y cuándo salir del proceso; speedup 7.36× con 8 contenedores | `proyecto-final` |
| 3 · avanzado | [3-avanzado-compose-escalado](./3-avanzado-compose-escalado) | **ejecutable**: `docker compose --scale worker=N` y un cliente que mide el speedup | Compose, balanceo en el cliente |

## Cómo elegir

| Tu cuello de botella es… | Nº de workers | Y si no basta… |
|--|--|--|
| **CPU** | ≈ `GOMAXPROCS` | más núcleos → más procesos/contenedores/máquinas |
| **E/S** (red, disco, BD) | muchos (cientos o miles) | limitado por el **recurso remoto**: usa un semáforo (5.1) y límites de tasa (5.3) |
| **memoria / ancho de banda** | 2-4 | trocea en bloques que quepan en caché; datos más compactos |
| **un lock** | menos | elimina el estado compartido (concepto 2) |

## Sigue con

Si tu trabajo es CPU, el [concepto 8](../08-medir-el-paralelismo) explica cuánto debes esperar de añadir núcleos (speedup y ley de Amdahl) y el [concepto 9](../09-repartir-el-trabajo), cómo repartirlo bien.
