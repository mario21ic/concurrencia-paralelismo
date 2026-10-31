# Ejercicio 2: `iter.Pull`

## Teoría

`range` es **push** (el iterador manda). Hay dos casos donde no sirve:

1. **Avanzar dos secuencias a la vez**: mezclar dos listas ordenadas, emparejar (`Zip`), comparar. Un `range` consume una secuencia entera antes de que puedas tocar la otra.
2. **Control manual**: "dame el siguiente cuando yo quiera", típico de parsers y *lookahead*.

```go
next, stop := iter.Pull(seq)
defer stop()
v, ok := next()    // ok=false cuando se agotó
```

### Cómo funciona

`Pull` ejecuta `seq` en una **corrutina**: una goroutine que se suspende y reanuda mediante traspaso directo de control con quien llama a `next`, sin pasar por el scheduler ni despertar otro hilo. No hay ejecución simultánea: mientras una corre, la otra espera. **Es cambio de contexto cooperativo, no paralelismo.**

### Reglas

- **Llama siempre a `stop()`** (con `defer`) si puedes abandonar antes del final. Si no, la corrutina queda suspendida para siempre y **el GC no la libera**: es una fuga de goroutine.
- `stop()` es idempotente y, tras agotarse la secuencia, innecesario pero inofensivo.
- `next` y `stop` no son seguras para uso concurrente desde varias goroutines.
- Un `panic` dentro del iterador se propaga a quien llamó a `next`.
- `iter.Pull2` hace lo mismo para `Seq2`.

### Dos aplicaciones del ejercicio

- **`Mezclar`** (merge de dos secuencias ordenadas, el paso central de merge sort y de uniones ordenadas): mantiene el "cabeza" de cada una y avanza la menor.
- **`Zip`**: llama a `next` en ambas y termina con la más corta.

Ambas devuelven un `Seq`, y hacen `defer stop()` de sus dos `Pull`: si el consumidor hace `break`, las corrutinas se liberan.

## Qué hace el programa

1. Uso básico de `Pull`.
2. Mezcla de dos secuencias ordenadas y `Zip`.
3. 100 `Pull` abandonados **sin** `stop()` frente a 100 con `stop()`.
4. Coste de `next()` frente a recibir de un channel sin buffer.

```bash
go run .
go test -race -count=2 .
```

## Resultado de referencia

```
mezcla ordenada: [1 2 3 4 7 8 9 10 11 12]
zip: (1,uno) (2,dos)

goroutines (corrutinas) fugadas por olvidar stop(): 100
goroutines fugadas llamando a stop():             0

1000000 llamadas a next(): 64ms (63ns por llamada)
1000000 recepciones de channel sin buffer: 123ms (123ns por recepción)
```

## Conclusiones

- Olvidar `stop()` fuga **una corrutina por `Pull`** (100 de 100), incluso tras `runtime.GC()`.
- `next()` cuesta ~63 ns; un channel sin buffer ~123 ns. Es **unas 2× más barato**, no órdenes de magnitud; pero sin necesitar scheduler ni segunda goroutine activa.
- Con `Pull` no hay paralelismo: el productor y el consumidor se alternan.

## Tests

`Mezclar` correcta (incluso con una secuencia vacía), `Zip` con la más corta, `break` sobre `Mezclar` no deja fugas (50 iteraciones) y propagación de panic.

## Ejercicios propuestos

1. Escribe `Igual[T](a, b iter.Seq[T]) bool` con `Pull`.
2. Implementa un *tokenizer* con *lookahead* usando `Pull`.
3. Escribe un test que falle si olvidas el `defer stop()` en `Mezclar`.
4. Usa `iter.Pull2` con `maps.All` para recorrer dos mapas a la vez.
