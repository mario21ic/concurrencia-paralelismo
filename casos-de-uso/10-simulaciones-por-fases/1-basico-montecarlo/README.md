# Caso 10.1 — Monte Carlo: estimar π en paralelo  ·  `básico`

> **Problema:** Estimar π lanzando puntos al azar en [0,1)² y contando cuántos caen en el cuarto de círculo. Cada muestra es independiente: paralelismo «vergonzosamente fácil»… con una trampa.

## Qué features usa y por qué

| Variante | Generador | Qué pasa |
|--|--|--|
| **compartido + Mutex** | un único `*rand.Rand` protegido con un cerrojo | los workers hacen cola por cada número |
| **global `math/rand/v2`** | `rand.Float64()` (nivel superior) | seguro entre goroutines y **sin cerrojo** (cada hilo tiene su estado), pero no se puede fijar la semilla |
| **local por worker** | `rand.New(rand.NewPCG(semilla, w))` en cada goroutine | sin compartir nada y **reproducible** |

Otras piezas: `sync.WaitGroup.Go`, un contador `dentro[w]` por worker (sin carreras) y un reparto del resto (`muestras % k`) para no perder muestras.

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ **Generador local por worker** cuando necesites resultados **reproducibles** (tests, depuración, ciencia): misma semilla y mismo `k` dan el mismo resultado. Usa un segundo parámetro distinto por worker (`NewPCG(semilla, w)`) para que los flujos sean independientes.
- ✅ **Global `math/rand/v2`** si solo necesitas aleatoriedad rápida y no reproducibilidad.
- ❌ **Nunca un generador compartido con mutex** en un bucle caliente: la contención lo hace más lento que no paralelizar.
- ❌ No uses `math/rand` para criptografía (claves, tokens): eso es `crypto/rand`.

## Cómo ejecutarlo

```bash
go run .
go test -race -v .
```

## Qué observar (salida real, Apple M1, Go 1.26.5)

```
GOMAXPROCS=8. π con 20 M muestras (mejor de 3).

generador                        k=1            k=8            π estimado (k=8)
compartido + Mutex               553ms          3.535s (0.2×)  3.14210
global math/rand/v2              221ms          48ms (4.6×)    3.14116
local por worker (PCG)           182ms          40ms (4.5×)    3.14173

Reproducibilidad con generador local, semilla 42 y k=8: 3.14173440 / 3.14173440
```

- **Con el mutex, 8 workers tardan 3.5 s: 6.4× MÁS lento que 1 worker (553 ms)**, y 88× más que la versión local con 8 (40 ms). El cálculo es independiente, pero el cerrojo lo vuelve secuencial *y además* pelea por él.
- Incluso con **1 solo worker** el mutex es ≈ 3× más lento (553 ms frente a 182 ms): bloquear y desbloquear sin contención también cuesta.
- Global y local escalan parecido (**4.6×** y **4.5×**, el techo habitual del M1 con 8 workers). La diferencia práctica es la **reproducibilidad**: con la local, dos ejecuciones dan el mismo `3.14173440`.
- Los tres estiman π ≈ 3.141 con 20 M de muestras (error típico ≈ 1.6/√n ≈ 0.0004).

## Errores típicos

- Compartir un `*rand.Rand` entre goroutines **sin** cerrojo: es una carrera (no es seguro), a diferencia de las funciones de nivel superior.
- Inicializar todos los workers con la **misma semilla**: todos generan la misma secuencia y el resultado no mejora al añadir muestras.
- Reparar el rendimiento del mutex con «más workers»: empeora.
- Esperar el mismo resultado exacto al cambiar `k` con generadores locales: cada `k` reparte las muestras en flujos distintos, así que sale otro valor (igual de válido).

## Profundiza en el curso

- [map-reduce: estado local y fusión final](../../../13-rendimiento/06-map-reduce)
- [contador fragmentado](../../../13-rendimiento/03-contador-fragmentado): mismo principio, evitar compartir lo que se actualiza mucho
- [mutex: lo que cuesta](../../../06-sync/01-mutex)
- [estado compartido: del mutex al fragmentado](../../02-estado-compartido/3-avanzado-contadores)
- [`sync.Pool`](../../../06-sync/06-pool): otra forma de dar «estado propio» a cada goroutine

## Prueba tú

1. Cambia `rand.NewPCG` por `rand.NewChaCha8` en la variante local y compara tiempos: ¿qué pagas por un generador de mejor calidad?
2. Estima el volumen de una esfera de 3 dimensiones y compáralo con `4π/3`.
3. Usa `GOMAXPROCS=1` con la variante del mutex y 8 workers: ¿es mejor o peor que con 8 núcleos?
