# Ejercicio 3: preemption cooperativa frente a asíncrona

## Teoría

Go usa planificación **expropiativa** (*preemptive*), pero con dos mecanismos:

| Mecanismo | Cómo funciona | Limitación |
|--|--|--|
| **Cooperativa** | el compilador inserta en el **prólogo de las funciones** una comprobación del puntero de pila (`morestack`). Para pedir preemption, `sysmon` pone el límite de la pila en un valor imposible: la siguiente llamada "falla" la comprobación y cede | solo actúa en **llamadas a funciones con comprobación de pila**: un bucle sin llamadas, o que llama a funciones *hoja* pequeñas, **no tiene puntos de cesión** |
| **Asíncrona** (Go 1.14+) | `sysmon` detecta una G que lleva **> 10 ms** en un P (`forcePreemptNS`) y envía una **señal** (`SIGURG`) al hilo; el manejador comprueba si el código está en un **punto seguro** y, si es así, desvía la ejecución al planificador | no puede interrumpir código en puntos no seguros (p. ej. dentro de ciertas secuencias del runtime/ensamblador/cgo) |

### Cuándo importaba (antes de 1.14)

Un `for {}` en una goroutine con `GOMAXPROCS=1` **colgaba el programa** (el GC no podía parar el mundo). Con la asíncrona ya no, pero la latencia sigue siendo ≈ 10-25 ms.

### La función hoja

El compilador **omite** el prólogo de pila en una función hoja con un marco diminuto (por debajo de `StackSmall`, 128 B). No hay punto de cooperación en ese código. Una función con un marco grande **sí** comprueba la pila y es un punto de preemption cooperativa.

### Implicaciones prácticas

- Un bucle de cálculo largo **sí** se interrumpe (señal), pero **la latencia de otras goroutines** (p. ej. un `time.Sleep(1ms)`) pasa de ≈ 1 ms a ≈ 25 ms mientras haya un acaparador en el mismo P.
- Si necesitas baja latencia con cálculo intensivo: más `GOMAXPROCS`, trocear el trabajo, o `runtime.Gosched()` en bucles críticos.
- `GODEBUG=asyncpreemptoff=1` desactiva la asíncrona: útil para depurar, **peligroso** en producción.

## Qué hace el programa

| Modo | Experimento |
|--|--|
| `-modo=comparar` | 6 configuraciones (3 tipos de acaparador × con/sin `asyncpreemptoff`): ¿despierta `main` de un `time.Sleep(10ms)`? Cada una en un proceso hijo con `GOMAXPROCS=1`, con un plazo de 3 s |
| `-modo=latencia` | `time.Sleep(1ms)` medido 30 veces sin acaparador y con un acaparador (bucle) |

```bash
go run . -modo=comparar
go run . -modo=latencia
```

## Resultados de referencia (Apple M1, macOS, Go 1.26.5)

```
configuración (GOMAXPROCS=1, un acaparador)          resultado
bucle sin llamadas, por defecto                      main despertó tras 23ms
bucle sin llamadas, asyncpreemptoff=1                BLOQUEADO (hubo que matar el proceso a los 3 s)
llama a función hoja pequeña, por defecto            main despertó tras 25ms
llama a función hoja pequeña, asyncpreemptoff=1      BLOQUEADO
llama a función de marco grande, por defecto         main despertó tras 25ms
llama a función de marco grande, asyncpreemptoff=1   main despertó tras 25ms   ← cooperativa

time.Sleep(1ms) medido 30 veces, GOMAXPROCS=1:
sin acaparador               mín=1.03ms   mediana=1.13ms   máx=1.19ms
con un acaparador (bucle)    mín=21.82ms  mediana=25.03ms  máx=25.1ms
```

## Conclusiones

- **Sin preemption asíncrona, un bucle sin llamadas o con una función hoja pequeña bloquea el programa para siempre**; con ella, `main` despierta a los ≈ 23-25 ms.
- **Una función con marco grande sí es un punto de cesión cooperativo:** funciona incluso con `asyncpreemptoff=1` (25 ms).
- **El coste en latencia es enorme:** un `Sleep(1ms)` pasa de 1.13 ms (mediana) a **25 ms** con un acaparador. La razón es el sondeo de `sysmon` (10 ms de `forcePreemptNS` más su periodo de sondeo).
- Lección de diseño: el runtime protege contra el **cuelgue**, no contra la **latencia**. Una goroutine de cálculo largo debería ceder (`Gosched`), trocearse, o correr en un P propio.

## Ejercicios propuestos

1. Añade una configuración con `runtime.Gosched()` cada 1 000 iteraciones y mide la latencia.
2. Mide cómo cambia la latencia con `GOMAXPROCS=2` (un P libre para `main`).
3. Compila con `-gcflags=-S` y localiza el prólogo `morestack` en `conMarcoGrande` y su ausencia en `hojaPequena`.
4. Prueba un bucle que llame a `time.Now()` en cada vuelta: ¿es un punto de cesión?
