# Ejercicio 2: captura de variables de bucle

## Teoría

Antes de Go 1.22, `for i := 0; ...` declaraba **una sola** `i` para todo el bucle. Un closure o goroutine que la capturaba veía el valor *actual* de esa única variable, normalmente el final. Era uno de los bugs más comunes de Go.

Desde Go 1.22 (siempre que `go.mod` declare `go 1.22` o superior), cada iteración tiene **su propia copia** de `i`:

```go
for i := 0; i < 5; i++ {
    go func() { fmt.Println(i) }()   // seguro desde 1.22
}
```

Esto vale para `for i := ...;;`, `for k, v := range ...` y `for i := range n`.

Cuidado: si declaras la variable **fuera** del `for` (`var i int; for i = 0; ...`) vuelve a ser una sola compartida, como antes. También sigue habiendo race si la variable es compartida por otro motivo.

## Qué hace el programa

Lanza 5 goroutines que emiten su `i` y muestra los valores ordenados, de tres formas.

```bash
go run .
```

## Resultado de referencia

```
semántica 1.22 (i por iteración):   [0 1 2 3 4]
variable compartida (estilo <1.22): [5 5 5 5 5]     <- todas ven el valor final
pasando el valor como argumento:    [0 1 2 3 4]
```

La segunda versión además tiene un data race (`go run -race .` lo marca).

## Conclusiones

- Con `go 1.22+` en `go.mod`, el patrón habitual es seguro y `i := i` ya no hace falta.
- La semántica depende de la versión declarada en **`go.mod`**, no de la versión del compilador instalado. Revisa que tu `go.mod` diga `go 1.22` o más.
- Es posible reproducir el bug declarando la variable fuera del bucle.

## Ejercicios propuestos

1. Cambia el `go` del `go.mod` a `1.21`, ejecuta la primera variante y observa que ahora se comporta como la segunda.
2. Repite el experimento con `for _, v := range []string{"a","b","c"}`.
3. Ejecuta con `-race` y localiza la línea del race en la segunda versión.
