# Ejercicio 8: generadores y fugas

## Enunciado

Implementar un generador de números de Fibonacci con un channel y asegurarse de que no filtra goroutines cuando el consumidor lee solo algunos valores.

## Teoría

Patrón **generador**: una función arranca una goroutine productora y devuelve `<-chan T`.

```go
func gen() <-chan int {
    ch := make(chan int)
    go func() { defer close(ch); for { ch <- next() } }()
    return ch
}
```

Problema: si el consumidor deja de leer tras `n` valores, el productor queda bloqueado en `ch <- ...` **para siempre**. Es una **fuga de goroutine** (goroutine leak): consume memoria y no la recupera el GC.

Solución: darle una vía de salida. Aquí un channel `done`; el productor usa `select`:

```go
select {
case ch <- a:
case <-done:
    return
}
```

En el módulo 5 sustituiremos `done` por `context.Context`.

Regla: **para cada goroutine que lances, debes poder responder "¿cuándo y cómo termina?"**.

## Cómo ejecutarlo

```bash
go run .
```

## Resultado de referencia

```
ingenuo: [0 1 1 2 3 5 8 13 21 34]
goroutines fugadas tras el ingenuo: 1
con done: [0 1 1 2 3 5 8 13 21 34]
goroutines fugadas en total: 1 (solo queda la del ingenuo)
```

## Conclusiones

- La versión ingenua deja 1 goroutine bloqueada para siempre.
- La versión con `done` termina limpiamente al cerrar `done`.
- `runtime.NumGoroutine()` es la forma más sencilla de detectar fugas en pruebas.

## Ejercicios propuestos

1. Escribe `take(done, in, n)` que devuelva solo los primeros `n` valores y cierre su salida.
2. Genera primos con el mismo patrón y encadénalo con `take`.
3. Escribe un test que falle si `NumGoroutine()` crece tras usar el generador ingenuo.
