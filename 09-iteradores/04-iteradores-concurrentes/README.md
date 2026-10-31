# Ejercicio 4: iteradores concurrentes

## Enunciado

Construir dos adaptadores que usan goroutines por dentro pero se consumen con un simple `range`:

- **`Prefetch(seq, buf)`**: el productor corre por delante del consumidor.
- **`ParMap(seq, workers, f)`**: aplica `f` en paralelo y entrega los resultados **en orden**.

## Teoría

Un iterador es síncrono por contrato, pero puede **ocultar** concurrencia. A cambio debe cumplir reglas estrictas (ver README del módulo): `yield` solo desde la goroutine llamante, no retornar con goroutines vivas, cancelar y esperar al hacer `break`, y propagar los panics.

### `Prefetch`: solapar productor y consumidor

```
productor ──► [buffer de N] ──► yield (en la goroutine del consumidor)
```

Si producir y consumir cuestan lo mismo, el tiempo total pasa de `n·(p+c)` a ~`n·max(p,c)`.

Detalles de implementación:
- Un `context` interno + `defer { cancel(); <-hecho }`: ocurra lo que ocurra (break, fin, panic del consumidor) el productor se detiene y **se espera** antes de retornar.
- El panic del productor se captura con `recover` y se **relanza en la goroutine que itera** una vez cerrado el canal.

### `ParMap`: map paralelo con orden

El problema de los pools (módulo 8) es que los resultados salen desordenados. Aquí:

```
origen ──► [por cada elemento: un "futuro" (chan de 1)] ──► cola ordenada ──► consumidor lee en orden
                 │ lanza goroutine que calcula f(a) y escribe en su futuro
```

- El consumidor lee los futuros **en el orden de encolado**, así que los resultados salen ordenados aunque se calculen desordenados.
- Dos límites: un **semáforo** de `workers` plazas acota los cálculos simultáneos; la **cola de futuros** (capacidad `workers`) acota los resultados pendientes y frena al origen si el consumidor es lento.
- Es **perezoso**: la fuente solo avanza lo que la contrapresión permite.
- Todas las goroutines (productor + cálculos) se cuentan en un `WaitGroup` que se espera antes de retornar.

> Un fallo real durante el desarrollo: la primera versión acotaba solo con la cola, de modo que `workers=1` permitía 2 cálculos simultáneos (1 en cola + 1 encolándose) y el test con `<= 5` lo dejaba pasar. Se añadió el semáforo y se endureció el test a `<= 4`. Lección: un límite se prueba con el valor exacto.

## Cómo ejecutarlo

```bash
go run .
go test -race -count=10 .
```

## Resultado de referencia

```
== Prefetch: 30 elementos, 10ms producir + 10ms consumir ==
  en serie:  650ms (≈ 600ms)
  prefetch:  340ms (≈ 300ms: se solapan)

== ParMap: 40 × 20ms ==
  1 worker:  840ms
  8 workers: 100ms (8.0x)

orden de salida de ParMap: [50 10 40 20 30]     <- entrada 5,1,4,2,3 (×10), orden preservado

break tras 3 resultados: la fuente produjo solo 4 de 1000000 elementos
goroutines fugadas: 0

panic de f recuperado por el consumidor: f falló con 2
```

## Conclusiones

- `Prefetch` reduce casi a la mitad el tiempo cuando productor y consumidor tardan lo mismo.
- `ParMap` escala linealmente (8.0× con 8 workers) y conserva el orden.
- El `break` detiene la fuente (solo 4 elementos producidos de un millón) y no deja goroutines vivas.
- Un panic en `f` llega al consumidor en lugar de matar el proceso.

## Tests (`-race -count=10`)

Orden y contenido, `break` sin fugas (30 iteraciones), propagación de panic (Prefetch y ParMap), orden con tiempos desiguales, **límite exacto de concurrencia (4)**, y perezosidad de la fuente.

## Ejercicios propuestos

1. Añade un parámetro `context.Context` para cancelar desde fuera, devolviendo `Seq2[B, error]`.
2. Implementa `ParMapDesordenado` (sin orden, más rápido si los tiempos son muy desiguales) y mide la diferencia.
3. Escribe `Lotes(seq, n)` y combínalo con `ParMap` para procesar por lotes.
4. Haz que `ParMap` propague el **primer** error de `f` (`f` devuelve `(B, error)`) cancelando el resto.
