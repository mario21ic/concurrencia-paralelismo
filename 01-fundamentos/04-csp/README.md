# Ejercicio 4: modelo CSP

## Teoría

**CSP (Communicating Sequential Processes, Hoare 1978):** un programa es un conjunto de procesos secuenciales independientes que se coordinan **enviándose mensajes** por canales, en lugar de compartir variables.

El lema de Go:

> *Don't communicate by sharing memory; share memory by communicating.*

En la práctica, el principio es **propiedad única del dato**: en cada momento solo una goroutine accede a un valor. Se logra:

- pasando el dato por un channel (el que lo envía deja de usarlo), o
- teniendo una goroutine "dueña" del estado que atiende peticiones.

Go **no** es CSP puro. Ofrece también `sync.Mutex` y `atomic`, y a menudo son la mejor opción:

| Usa channels cuando... | Usa mutex/atomic cuando... |
|--|--|
| transfieres propiedad de datos | proteges un estado compartido simple (contador, caché) |
| distribuyes trabajo / pipelines | la sección crítica es corta |
| coordinas eventos o cancelación | priorizas rendimiento |

Un channel **sin buffer** es además un punto de **sincronización** (rendezvous): el emisor y el receptor se encuentran. Esto crea una relación *happens-before* (ver ejercicio 6).

## Qué hace el programa

1. Un contador con `Mutex` (memoria compartida).
2. El mismo contador con una goroutine dueña del estado y mensajes por channel (CSP).
3. Un ping-pong entre dos goroutines con channels sin buffer.

```bash
go run .
```

## Resultado de referencia

```
Mutex: 100000 (esperado 100000) en 8.97ms
CSP:   100000 (esperado 100000) en 27.807ms

ping-pong, valor: 1 ... 5
```

## Conclusiones

- Ambos son correctos; **ninguno tiene data races**.
- Para un contador simple, el mutex es ~3x más rápido: cada mensaje de channel cuesta más que un lock sin contención. Elegir CSP aquí es por claridad de diseño, no por velocidad.
- CSP brilla cuando el estado es complejo y quieres garantizar que *una* goroutine lo manipula.

## Ejercicios propuestos

1. Agrega `sync/atomic` como tercera versión y compara tiempos.
2. Convierte el contador CSP para que también atienda `get` mediante un channel de peticiones con un channel de respuesta.
3. Implementa un "banco" con 2 cuentas y transferencias: primero con mutex, luego con una goroutine dueña. ¿Cuál evita mejor el deadlock?
