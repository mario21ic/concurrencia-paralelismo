# Ejercicio 7: reproducir y arreglar un data race

## Enunciado

1000 goroutines añaden su id a un slice compartido con `append`. El resultado debería tener 1000 elementos.

## Teoría

`out = append(out, i)` parece una sola operación, pero es: leer `out` (ptr, len, cap) → posiblemente reservar un array nuevo → escribir el elemento → **reasignar** `out`. Dos goroutines intercaladas pueden escribir en la misma posición o pisar la reasignación del slice, **perdiendo elementos**. Es un data race.

Cuatro formas de arreglarlo, de la más simple a la más estructurada:

| Solución | Idea | Cuándo |
|--|--|--|
| `mutex` | un lock protege el slice | secciones críticas cortas, estado compartido |
| `indice` | slice preasignado, cada goroutine escribe en su índice | tamaño conocido; la más rápida, sin locks |
| `canal` | las goroutines envían y **una sola** (dueña del slice) hace `append` | estilo CSP; el orden de llegada no es el del índice |
| (`atomic`) | no aplica a slices; sí a contadores simples | |

## Cómo ejecutarlo

```bash
go run . -modo=bug             # pierde elementos
go run -race . -modo=bug       # el detector señala la race
go run . -modo=mutex
go run . -modo=indice
go run . -modo=canal
go run -race . -modo=indice    # sin warnings
```

## Resultado de referencia

```
bug:    909 elementos (esperados 1000)
mutex:  1000 elementos (esperados 1000)
indice: 1000 elementos (esperados 1000)
canal:  1000 elementos (esperados 1000)

WARNING: DATA RACE
Read at 0x00c000012188 by goroutine 9:
  runtime.growslice()
  main.bug.func1() main.go:23
Previous write at 0x00c000012188 by goroutine 7:
  main.bug.func1() main.go:23
```

## Método para arreglar una race

1. Reproducir con `-race` y leer **ambos** stacks (lectura y escritura previa).
2. Identificar qué dato se comparte y quién debería ser su dueño.
3. Elegir: proteger (mutex/atomic), dividir (cada goroutine su parte) o transferir (channel).
4. Volver a ejecutar con `-race` **varias veces** (`go test -race -count=100`).

## Ejercicios propuestos

1. Escribe un test `TestAppend` que falle con `go test -race -count=20` para `bug` y pase para las demás.
2. Repite el ejercicio con un `map[int]int` y observa el error `fatal error: concurrent map writes`.
3. Compara el rendimiento de las 3 soluciones con `testing.B` (módulo 13 lo profundiza).
