# Ejercicio 3: pipeline con cancelación

## Teoría

Un **pipeline** encadena etapas; cada una es una goroutine que lee de un channel de entrada, transforma y escribe en uno de salida. Permite que las etapas trabajen **a la vez** sobre elementos distintos (como una cadena de montaje).

```
Contar ──► Filtrar(pares) ──► Mapear(cuadrado) ──► Tomar(5) ──► consumidor
```

### Contrato de cada etapa

1. Recibe `<-chan A` y `ctx`; devuelve `<-chan B`.
2. **Cierra su salida** cuando termina (`defer close(out)`).
3. Termina cuando su entrada se cierra **o** `ctx` se cancela.
4. Todo envío va en `select` con `ctx.Done()`.

### El problema de las fugas

La contrapresión fluye **hacia atrás**: si `Tomar` deja de leer tras 5 elementos, `Mapear` queda bloqueado enviando, luego `Filtrar`, luego `Contar`. Solo una **cancelación** (`ctx`) libera a todas. El consumidor que acaba antes de tiempo debe `defer cancel()`.

### Errores en un pipeline

Opciones:
1. **Error como dato** (`Par[T]{Val, Err}`): cada etapa lo reenvía y la última decide. Es lo implementado aquí (`Parsear`).
2. Un channel de errores aparte.
3. `errgroup`: el primer error cancela el contexto compartido (módulo 10).

### Cuándo conviene un pipeline

Cuando las etapas son costosas e independientes, o cuando el flujo es potencialmente infinito. Con etapas triviales, el coste de comunicar por channel (≈ 100–300 ns por salto) supera al del trabajo: mejor una función normal.

## Qué hace el programa

1. Pipeline sobre una fuente infinita; toma 5 y cancela.
2. El mismo pipeline **sin** cancelación: cuenta las goroutines fugadas.
3. Errores como datos.

```bash
go run .
go test -race -count=3 .
```

## Resultado de referencia

```
primeros 5 cuadrados de pares: 4 16 36 64 100
goroutines activas antes de cancelar: 3 (la fuente sigue produciendo)
goroutines activas tras cancelar:     0

sin cancelar, goroutines fugadas:    3 (3 etapas bloqueadas para siempre)

parseo con errores:
  valor: 1
  valor: 2
  error: strconv.Atoi: parsing "x": invalid syntax
  valor: 4
```

## Conclusiones

- Aunque `Tomar` ya terminó, las 3 etapas anteriores siguen vivas hasta que se cancela.
- Sin cancelar quedan 3 goroutines fugadas para siempre.
- Un error de parseo no detiene el flujo.

## Ejercicios propuestos

1. Añade una etapa `Paralelizar(ctx, in, n, f)` que use fan-out/fan-in internamente (ejercicio 2) y compáralo con la etapa serie.
2. Escribe `Lotes` (agrupar de N en N) como etapa.
3. Haz que el pipeline se detenga al primer error con `WithCancelCause`.
4. Mide el coste por etapa con un benchmark: ¿cuántos ns añade cada salto?
