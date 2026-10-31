# Ejercicio 1: iteradores y `range over func`

## Teoría

```go
func Contar(desde int) iter.Seq[int] {
    return func(yield func(int) bool) {
        for i := desde; ; i++ {
            if !yield(i) { return } // false => el consumidor salió del bucle
        }
    }
}
```

### Composición perezosa

`Map`, `Filter`, `Take` son **adaptadores**: reciben un `Seq` y devuelven otro. Nada se ejecuta hasta el `range` final, y cada elemento atraviesa toda la cadena **antes** de pedir el siguiente (como un pipeline de una sola goroutine):

```
Contar ──► Filter(par) ──► Map(cuadrado) ──► Take(5) ──► range
```

Por eso funcionan sobre secuencias **infinitas**: `Take` o un `break` las detienen.

### `Seq` vs `Seq2`

`Seq[V]` produce un valor; `Seq2[K, V]` produce dos (índice/valor, clave/valor, valor/error). `for i, v := range seq2`.

### El contrato de `yield`

| Situación del consumidor | `yield` devuelve |
|--|--|
| el cuerpo termina normalmente | `true` |
| `break`, `return`, `goto` fuera, `panic` | `false` |

Un iterador que **ignora el `false`** y sigue llamando a `yield` provoca en tiempo de ejecución:

```
runtime error: range function continued iteration after function for loop body returned false
```

### Limpieza con `defer`

El `defer` del iterador se ejecuta al terminar la función iteradora, **también** tras un `break`. Es el patrón para cerrar archivos, cursores de base de datos o conexiones:

```go
func Lineas(ruta string) iter.Seq2[string, error] {
    return func(yield func(string, error) bool) {
        f, err := os.Open(ruta)
        if err != nil { yield("", err); return }
        defer f.Close()          // se cierra aunque el consumidor haga break
        ...
    }
}
```

### Atajos de la stdlib

`slices.Collect(seq)` → slice; `slices.Sorted(maps.Keys(m))` → claves ordenadas; `slices.Chunk(s, n)` → lotes; `strings.SplitSeq` (1.24) → sin slice intermedio.

## Qué hace el programa

1. Composición perezosa sobre una fuente infinita.
2. `Seq2` con índice.
3. Biblioteca estándar: `maps.Keys`, `slices.Chunk`, `strings.SplitSeq`, `slices.Collect`.
4. `defer` del iterador al hacer `break`.
5. Un iterador "malo" que ignora `false` y su panic.

```bash
go run .
go test -race .
```

## Resultado de referencia

```
primeros 5 cuadrados de pares: 4 16 36 64 100
Fibonacci < 100: 0 1 1 2 3 5 8 13 21 34 55 89
claves ordenadas: [a b c]
[1 2 3] [4 5 6] [7] <- slices.Chunk
a b c <- strings.SplitSeq

break dentro del bucle:
  [iterador] abriendo recurso
  consumidor recibe 1
  consumidor recibe 2
  [iterador] cerrando recurso

iterador malo -> runtime error: range function continued iteration after function for loop body returned false
```

## Conclusiones

- Los iteradores permiten secuencias infinitas y composición sin crear slices intermedios.
- La limpieza del iterador ocurre aunque el consumidor salga antes.
- Respetar el valor de `yield` no es opcional.

## Tests

Composición correcta, **perezosidad** (con `Take(3)` solo se calculan 3 elementos), `defer` ejecutado tras `break`, y panic del iterador malo.

## Ejercicios propuestos

1. Escribe `Zip`, `Chunk` y `SkipWhile` como adaptadores.
2. Haz `Lineas(ruta)` como en la teoría y verifica con `lsof` o un contador que el archivo se cierra tras un `break`.
3. Implementa un iterador de árbol binario en orden (recursivo) con `yield`.
4. Calcula primos con un iterador infinito y toma los 20 primeros.
