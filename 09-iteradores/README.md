# Módulo 9. Iteradores y concurrencia

**Objetivo:** entender los iteradores de Go (`iter.Seq`, `range over func`, `iter.Pull`), cuándo sustituyen a los channels, y cómo escribir iteradores que usan concurrencia sin romper su contrato.

## Ejercicios

| # | Carpeta | Tema |
|---|---------|------|
| 1 | [01-iter-basico](01-iter-basico) | `iter.Seq`/`Seq2`, `range over func`, composición perezosa, contrato de `yield` |
| 2 | [02-iter-pull](02-iter-pull) | `iter.Pull`: mezclar y emparejar secuencias, fugas por olvidar `stop` |
| 3 | [03-iteradores-vs-canales](03-iteradores-vs-canales) | Mismo pipeline con bucles, iteradores, `Pull` y channels (benchmarks) |
| 4 | [04-iteradores-concurrentes](04-iteradores-concurrentes) | `Prefetch` y `ParMap` ordenado: concurrencia dentro de un iterador |
| 5 | [05-canales-e-iteradores](05-canales-e-iteradores) | Convertir channel ↔ iterador y paginación perezosa con errores |

```bash
cd 09-iteradores/01-iter-basico && go run . && go test -race .
```

## Resumen teórico

### Qué es un iterador

Desde Go 1.23 un iterador es una **función** que recibe un callback `yield` y lo invoca con cada elemento:

```go
type Seq[V any]     func(yield func(V) bool)
type Seq2[K, V any] func(yield func(K, V) bool)
```

El cuerpo de un `for ... range` sobre esa función se convierte en el callback:

```go
for v := range seq { ...; if cond { break } }
// el compilador lo traduce a: seq(func(v T) bool { ...; return !cond })
```

- `yield` devuelve `false` cuando el consumidor ha hecho `break`, `return`, `goto` o ha entrado en `panic`: el iterador **debe parar** inmediatamente. Si sigue llamando a `yield`, el runtime lanza un panic (`range function continued iteration after function for loop body returned false`).
- Los `defer` del iterador se ejecutan siempre → es el sitio para **liberar recursos** (archivos, cursores).
- Son **perezosos**: no calculan nada hasta que alguien itera.

### Biblioteca estándar

`slices.Values/All/Collect/Sorted/Chunk`, `maps.Keys/Values/All/Collect`, `strings.SplitSeq/Lines/FieldsSeq` (1.24), `bytes.*Seq`. Los iteradores de la stdlib cubren la mayoría de necesidades sin escribir uno propio.

### `iter.Pull`

`range` es **push**: el iterador empuja los valores. A veces se necesita **pull**: avanzar a demanda, o recorrer **dos** secuencias a la vez (mezclar, zip). `iter.Pull(seq)` devuelve `next` y `stop`:

```go
next, stop := iter.Pull(seq)
defer stop()               // OBLIGATORIO si puedes abandonar antes del final
v, ok := next()
```

Por dentro corre `seq` en una **corrutina** (una goroutine con traspaso directo de control, sin pasar por el scheduler). Cada `next()` es un traspaso síncrono: **no hay paralelismo**. Si no llamas a `stop()` y abandonas, la corrutina queda suspendida para siempre: fuga.

### Iteradores frente a channels

| | Iterador (`Seq`) | Channel |
|--|--|--|
| Ejecución | síncrona, **misma goroutine** | otra goroutine |
| Paralelismo | **no** | sí |
| Coste por elemento | ~1 ns (se inlinea) | 100–350 ns |
| Cancelación | `break` (automática) | `ctx`/`done` explícito |
| Fugas posibles | solo con `Pull` sin `stop` | muchas (productor bloqueado) |
| Contrapresión | implícita (el productor *es* el consumidor) | por buffer |
| Varios consumidores | no (un solo `range`) | sí (fan-out) |

Regla práctica: **si no necesitas concurrencia, usa un iterador; si la necesitas, usa goroutines + channels (o un iterador que las oculte con cuidado, ejercicio 4)**.

### Contrato de un iterador concurrente

1. `yield` se llama **siempre desde la goroutine que recibió la llamada** al iterador, y nunca de forma concurrente.
2. El iterador **no retorna** hasta que sus goroutines internas terminaron.
3. Si `yield` devuelve `false`: cancelar y **esperar** a esas goroutines.
4. Un `panic` en una goroutine interna debe **propagarse** a quien itera (si no, tumba el proceso).
5. Los errores se devuelven como segundo valor (`Seq2[T, error]`).

## Preguntas de repaso

1. ¿Qué devuelve `yield` y qué debe hacer el iterador si es `false`?
2. ¿Qué ocurre si un iterador ignora ese valor?
3. ¿Por qué un `defer` dentro del iterador es útil?
4. ¿Cuándo necesitas `iter.Pull` en lugar de `range`?
5. ¿Qué pasa si olvidas `stop()`? ¿Se arregla con el GC?
6. ¿Por qué un pipeline con iteradores es mucho más barato que uno con channels, y qué pierdes a cambio?
7. ¿Por qué `yield` no puede llamarse desde una goroutine distinta a la del consumidor sin cuidado?
8. ¿Cómo conservas el orden en un map paralelo?

## Lecturas

- [Range Over Func and Iterators in Go 1.23](https://go.dev/blog/range-functions)
- [Go 1.23 release notes](https://go.dev/doc/go1.23#iterators)
- Documentación de [`iter`](https://pkg.go.dev/iter), [`slices`](https://pkg.go.dev/slices) y [`maps`](https://pkg.go.dev/maps)
- *Coroutines for Go* — Russ Cox (explica cómo se implementa `iter.Pull`)
