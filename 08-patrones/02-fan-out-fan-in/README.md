# Ejercicio 2: fan-out / fan-in

## Teoría

- **Fan-out:** varias goroutines leen del **mismo** channel de entrada. Cada valor lo recibe **exactamente una**; es reparto de trabajo (el scheduler decide quién coge qué).
- **Fan-in:** varios channels se **combinan** en uno solo.

```
                 ┌─► worker 1 ─┐
generador ──────►├─► worker 2 ─┼──► Merge ──► consumidor
  (un origen)    └─► worker N ─┘
```

### `Merge` (fan-in)

Una goroutine por channel de entrada reenvía a `out`. Un **coordinador** espera a todas con `WaitGroup` y es el **único que cierra `out`** (nadie más puede, porque hay varios emisores). Todo envío va con `select` sobre `ctx.Done()`.

### Propiedades

- **Orden:** no se conserva (ni dentro de un worker respecto a otros). Si lo necesitas, etiqueta con el índice y reordena después, o usa el pool del ejercicio 1.
- **Balanceo:** al leer todos del mismo channel, un worker rápido toma más trabajo que uno lento (balanceo natural, *work sharing*).
- **Fan-out ≠ pub/sub:** aquí cada valor se procesa **una vez**; en pub/sub, **todos** reciben todo (ejercicio 7).

## Qué hace el programa

Eleva al cuadrado 40 números con 1, 2, 4 y 8 workers (10 ms por elemento), comprueba la suma y muestra el desorden de la salida.

```bash
go run .
go test -race -count=3 .
```

## Resultado de referencia

```
suma de cuadrados esperada: 22140

 workers       tiempo    suma ok
       1        437ms       true
       2        218ms       true
       4        110ms       true
       8         55ms       true

primeros 10 resultados con 4 workers (¡no están en orden!): [16 9 4 1 64 25 49 36 121 81]
```

## Conclusiones

- El tiempo se divide casi exactamente por el número de workers.
- La suma coincide siempre: ningún valor se pierde ni se duplica.
- El orden de salida es arbitrario, aunque agrupado de forma curiosa por worker.

## Tests

`Merge` entrega todo; `FanOut` entrega cada valor exactamente una vez (500 valores, 7 workers); cancelación sin fugas.

## Ejercicios propuestos

1. Propaga el índice con cada valor y reordena la salida con un buffer de reordenación.
2. Haz que un worker "lento" (tarda 5×) reciba menos trabajo y verifícalo contando.
3. Implementa `Merge` para un número variable de entradas usando `reflect.Select` y compara.
