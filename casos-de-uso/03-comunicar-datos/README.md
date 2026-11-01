# Concepto 3 · Comunicar datos entre goroutines

> **Pregunta guía:** Una goroutine produce datos y otras los consumen. ¿Cómo los paso sin compartir memoria, y cómo monto un flujo de varias etapas?

*«No comuniques compartiendo memoria; comparte memoria comunicando.»* Un **channel** transfiere el dato (y su propiedad) de una goroutine a otra, y además **sincroniza**. Es la herramienta para **flujos**; el mutex es la herramienta para **estado**.

## Escalera de casos

| Nivel | Caso | Idea | Feature principal |
|--|--|--|--|
| 1 · básico | [1-basico-productor-consumidor](./1-basico-productor-consumidor) | productor → consumidor; el buffer como amortiguador | `chan`, `close`, `range` |
| 2 · intermedio | [2-intermedio-pipeline](./2-intermedio-pipeline) | leer → parsear → agregar con cancelación | etapas con `chan` + `context` |
| 3 · avanzado | [3-avanzado-fan-out-ordenado](./3-avanzado-fan-out-ordenado) | repartir entre N workers y recuperar el orden de entrada | fan-out / fan-in, ventana de contrapresión |

## Cómo elegir

| Quieres… | Usa |
|--|--|
| pasar valores de una goroutine a otra | `chan T` |
| desacoplar el ritmo de productor y consumidor | `chan T` **con buffer** |
| avisar de «terminé» / «cancela» a muchos | **cerrar** un channel (`close`) o `context` |
| encadenar etapas | pipeline: cada etapa lee de la anterior y devuelve `<-chan` |
| repartir trabajo entre N workers | fan-out (N lectores del mismo channel) |
| recoger de varios channels en uno | fan-in (`select` o una goroutine por origen) |
| esperar al primero que responda / con plazo | `select` (módulo 4) |
| recorrer una secuencia **sin** concurrencia | un **iterador** `iter.Seq`, no un channel (más simple y sin fugas) |
