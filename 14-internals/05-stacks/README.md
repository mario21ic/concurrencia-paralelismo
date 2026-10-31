# Ejercicio 5: los stacks de las goroutines

## Teoría

Cada goroutine tiene su **propio stack**, que empieza en **≈ 2 KB** (mínimo del runtime) y se gestiona dinámicamente: es la razón de que se puedan tener millones de goroutines. Un hilo del SO reserva ≈ 8 MB (virtual) de entrada.

### Crecimiento por copia (*contiguous stacks*)

1. El prólogo de cada función comprueba si el stack alcanza (`morestack`).
2. Si no, el runtime **reserva uno del doble de tamaño**, **copia** el contenido y **ajusta todos los punteros** que apuntaban al stack viejo (los de las variables locales, los marcos, los punteros guardados en el propio stack), y libera el viejo.
3. Se reanuda la función como si nada.

Consecuencias:

- **Las direcciones de las variables locales cambian** al crecer el stack. Por eso Go no permite guardar un puntero al stack en un lugar que el runtime no conozca: un `uintptr` (no es un puntero para el GC ni para el ajuste), memoria C (cgo), etc. Un puntero a una variable local **no puede escapar** a una estructura fuera del alcance del runtime.
- Las copias son **O(tamaño)**: crecer de 2 KB a 20 MB implica copias sucesivas (2→4→8…) y cada una cuesta. Una recursión profunda **la primera vez** es más lenta que la segunda en la misma goroutine (el stack ya es grande).

### Reducción por el GC

El recolector **reduce** el stack a la mitad en cada ciclo si la goroutine usa **menos de ¼** de lo reservado (`shrinkstack`). Así una goroutine que hizo una recursión profunda puntual no retiene memoria para siempre.

### Límite y desbordamiento

- Límite por defecto: **1 GB** en 64 bits (`debug.SetMaxStack`). Superarlo es un `fatal error: stack overflow`: **no se puede recuperar con `recover`** (mata el proceso).
- Recursión infinita: llega al límite tras copiar stacks cada vez mayores (gasta memoria y tiempo antes de morir).

### Coste de memoria de una goroutine

≈ 2 KB de stack + el descriptor `g` (≈ 400 B) + el `sudog`/estructuras si está bloqueada. Un millón de goroutines bloqueadas ≈ 2.5-3 GB.

## Qué hace el programa

| Experimento | Qué mide |
|--|--|
| A | dirección de una variable local antes y después de una recursión profunda en la misma goroutine |
| B | tiempo de la 1.ª y la 2.ª ejecución de una recursión de 200 000 niveles en la misma goroutine |
| C | `StackInuse` con 20 goroutines que han hecho recursión profunda y siguen vivas, antes y después de 2 GC |
| D | 100 000 goroutines bloqueadas: `StackInuse` y bytes por goroutine |
| `-modo=desborde` | recursión infinita con `SetMaxStack(20 MB)`: `fatal error: stack overflow` |

```bash
go run .
go run . -modo=desborde      # termina con un fatal error (esperado)
```

## Resultados de referencia (Apple M1, Go 1.26.5)

```
A) dirección de la variable local antes: 0x5924de406f48 | después: 0x5924e045ff48   (cambió: true)
B) profundidad 200000: 1.ª vez 15.65 ms | 2.ª vez 1.81 ms  (8.7× más lenta la primera: copias sucesivas)
C) stack en uso: base 0.3 MB | 20 goroutines con recursión profunda 336.0 MB | tras 2 GC (siguen vivas) 84.3 MB
D) 100 000 goroutines bloqueadas: 204.9 MB de stack → 2049 bytes por goroutine (mínimo del runtime: 2048)

-modo=desborde: runtime: goroutine stack exceeds 20971520-byte limit → fatal error: stack overflow
```

## Conclusiones

- **El stack se mueve:** la variable local cambió de dirección. El runtime reajustó todos los punteros; por eso es seguro en Go y por eso no se pueden esconder punteros al stack.
- **Crecer cuesta:** la primera recursión profunda fue **8.7× más lenta** que la segunda (15.65 ms frente a 1.81 ms): el trabajo extra son las copias sucesivas.
- **El GC devuelve memoria:** 20 goroutines con recursión profunda ocuparon 336 MB; tras dos GC, 84 MB (cuatro veces menos) aunque las goroutines seguían vivas.
- **Una goroutine inactiva cuesta ≈ 2 KB de stack** (2 049 bytes por goroutine medido): 100 000 goroutines ≈ 205 MB. Por eso son tan baratas comparadas con los hilos.
- El desbordamiento de stack es **fatal e irrecuperable**: limita la profundidad de las recursiones sobre datos externos.

## Ejercicios propuestos

1. Mide el stack real de una goroutine con `runtime/metrics` (`/memory/classes/heap/stacks:bytes`) mientras crece la recursión.
2. Provoca el desborde con el límite por defecto de 1 GB y mide cuánto tarda y cuánta memoria consume antes de morir.
3. Comprueba con `go build -gcflags=-m` qué variables **escapan** al heap y cuáles quedan en el stack.
4. Observa con `GODEBUG=gcshrinkstackoff=1` qué pasa en el experimento C si se desactiva la reducción.
