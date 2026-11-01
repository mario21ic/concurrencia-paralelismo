# Caso 3.3 — Fan-out / fan-in conservando el orden  ·  `avanzado`

> **Problema:** Transformar un flujo (p. ej. fotogramas) con 4 workers. Los workers terminan en desorden, pero el cliente necesita los resultados en el orden original. Un fan-in simple los mezclaría.

## Qué features usa y por qué

| Feature de Go | Papel en este caso |
|--|--|
| Fan-out | N workers leen del mismo channel de trabajo |
| Numerar + reordenar | el recolector guarda los adelantados en un mapa y emite en orden |
| Ventana (`chan struct{}` de 2N) | acota cuántos elementos están en vuelo: un elemento lento no hace crecer la memoria sin límite (**contrapresión**) |
| `context` | cancelación en las tres etapas |
| `goleak` + `synctest` | tests de orden, tiempo y fugas |

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ Hay paralelismo útil **y** el orden importa (vídeo, logs, resultados paginados).
- ✅ Cada elemento tarda un tiempo apreciable (vale la pena repartir).
- ❌ El orden no importa → fan-in simple, más barato.
- ❌ Elementos casi instantáneos: la coordinación cuesta más que el trabajo.

## Cómo ejecutarlo

```bash
go run .
go test -race -v .
```

## Qué observar (salida real, Apple M1, Go 1.26.5)

```
resultados (en orden de entrada): [0 1 4 9 16 25 36 49 64 81 100 121]
tardó 110ms (secuencial: 210ms)
```

## Errores típicos

- Reordenar **sin ventana**: si el elemento 0 es lento, el mapa de pendientes crece sin límite.
- Que `Procesar` se crea dueño de la **fuente**: quien crea la fuente es quien debe poder detenerla.
- Perder elementos al cancelar: decide si se descartan o se terminan (aquí: se abandonan).
- Usar el número de worker como orden (no hay relación).

## Profundiza en el curso

- [fan-out / fan-in](../../../08-patrones/02-fan-out-fan-in)
- [más patrones de channels](../../../08-patrones/08-or-done-tee-bridge)
- [cola acotada y pérdidas](../../../15-diseno-avanzado/04-barbero)
- [cuándo agrupar en lotes](../../../13-rendimiento/04-batching)
- [tests con tiempo virtual](../../../11-testing/03-synctest-basico)

## Prueba tú

1. Cambia la ventana a `workers` y a `10*workers` y mide el efecto.
2. Haz una versión que NO conserve el orden y compara el tiempo.
3. Añade un contador de cuántos pendientes hay en el mapa como métrica.
