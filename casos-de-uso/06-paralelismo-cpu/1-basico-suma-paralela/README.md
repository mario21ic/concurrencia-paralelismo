# Caso 6.1 — Suma paralela de un slice  ·  `básico`

> **Problema:** Sumar 10 millones de números. Es CPU pura (no hay esperas): la única forma de acelerarla es usar más núcleos a la vez.

## Qué features usa y por qué

| Feature de Go | Papel en este caso |
|--|--|
| Particionar en trozos contiguos | cada worker recibe `xs[desde:hasta]` |
| Acumulador **local** por worker | `parciales[i]`: cada goroutine escribe en SU posición, sin cerrojos |
| `sync.WaitGroup.Go` | esperar a los workers |
| Combinar al final | sumar los `k` parciales (secuencial y barato) |

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ Cálculo CPU puro, datos grandes e independientes por trozo.
- ✅ Siempre **midiendo**: el speedup no está garantizado.
- ❌ Pocos datos: lanzar goroutines cuesta más que el cálculo (con n = 1 000 es ≈ 5× **más lento**).
- ❌ Operaciones limitadas por el **ancho de banda de memoria** (como esta suma): con 10 M de elementos el máximo fue ≈ 1.9×, no 8×.

## Cómo ejecutarlo

```bash
go run .
go test -race -v .
```

## Qué observar (salida real, Apple M1, Go 1.26.5)

```
GOMAXPROCS=8. Tiempo (mejor de 7) y aceleración frente a la versión secuencial:

n = 1000       secuencial 333ns      | k=2 792ns      (0.4×) | k=4 1.125µs    (0.3×) | k=8 1.875µs    (0.2×)
n = 100000     secuencial 33.584µs   | k=2 27.416µs   (1.2×) | k=4 23.625µs   (1.4×) | k=8 22.583µs   (1.5×)
n = 10000000   secuencial 3.385833ms | k=2 4.552916ms (0.7×) | k=4 1.773292ms (1.9×) | k=8 1.805416ms (1.9×)
```

## Errores típicos

- Paralelizar **sin medir**: más goroutines no siempre es más rápido.
- Escribir todos los workers en la misma variable `suma` (carrera, o mutex que serializa).
- Más workers que núcleos para trabajo CPU: solo compiten.
- Particionar mal (trozos desiguales) y que un worker retrase a todos.

## Profundiza en el curso

- [la ley de Amdahl](../../../01-fundamentos/05-amdahl)
- [speedup, eficiencia y sus techos](../../../13-rendimiento/01-escalabilidad)
- [map-reduce en profundidad](../../../13-rendimiento/06-map-reduce)
- [CPU-bound frente a E/S-bound](../../../01-fundamentos/02-cpu-vs-io-bound)
- [`GOMAXPROCS` y `NumCPU`](../../../02-goroutines/04-runtime-info)

## Prueba tú

1. Añade `n = 100 M` si tu memoria lo permite: ¿cambia el techo?
2. Cambia la suma por un cálculo caro por elemento (p. ej. `math.Sqrt` 100 veces): ¿ahora sí escala?
3. Fija `GOMAXPROCS=1` y repite: ¿qué ocurre con `k=8`?
