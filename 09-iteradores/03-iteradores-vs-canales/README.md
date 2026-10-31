# Ejercicio 3: iteradores frente a channels

## Enunciado

Implementar el mismo pipeline —filtrar pares, duplicar, sumar sobre 100 000 enteros— de cinco maneras y medir el coste.

## Teoría

- **Bucle:** línea base.
- **Iteradores:** las etapas son funciones que se llaman entre sí en **una sola goroutine**; el compilador puede *inlinear* los callbacks, de modo que el coste se aproxima al del bucle.
- **`iter.Pull`:** cada `next()` es un traspaso a una corrutina.
- **Channels:** una goroutine por etapa y una sincronización por elemento y por etapa (sin buffer: cada envío exige que el receptor esté listo; con buffer se amortiza).

## Cómo ejecutarlo

```bash
go test -bench . -benchmem -run xxx
go test -race .
```

## Resultados de referencia (100 000 elementos por operación, 8 núcleos)

```
BenchmarkBucle-8               64.5 µs/op      0 allocs
BenchmarkIteradores-8          65.6 µs/op      0 allocs
BenchmarkIteradoresPull-8      3.58 ms/op      9 allocs
BenchmarkCanalesSinBuffer-8    35.1 ms/op      6 allocs
BenchmarkCanalesBuffer64-8     10.2 ms/op      6 allocs
```

Por elemento de entrada (÷ 100 000):

| Versión | Coste por elemento | Relativo al bucle |
|--|--|--|
| Bucle | 0.65 ns | 1× |
| Iteradores | 0.66 ns | **1.0×** |
| Pull (por `next()`, 50 000 salidas) | ~72 ns | ~55× |
| Channels sin buffer | 351 ns | ~540× |
| Channels con buffer 64 | 102 ns | ~160× |

## Conclusiones

- **Los iteradores son prácticamente gratis:** el compilador los fusiona con el bucle (misma velocidad, 0 asignaciones).
- Los channels cuestan **centenares de veces más** por elemento. Para trabajo por elemento de unos pocos nanosegundos, un pipeline con goroutines es **mucho más lento** que el bucle secuencial, aunque use todos los núcleos.
- El buffer de 64 rebaja el coste ~3.4×, porque amortiza la sincronización.
- `Pull` queda en medio (≈ 72 ns por `next`), más barato que un channel pero lejos del `range`.
- **Criterio:** los channels y la concurrencia se justifican cuando cada elemento tiene un coste **significativo** (I/O, cálculo pesado, microsegundos o más), no para operaciones triviales. Si no hace falta concurrencia, usa iteradores.

## Ejercicios propuestos

1. Sube el coste por elemento (un cálculo de ~10 µs) y busca el punto de equilibrio en el que los channels con paralelismo (varios workers) superan al bucle.
2. Añade una versión con `ParMap` del ejercicio 4.
3. Mide con `-cpu 1,2,4,8` y observa qué versiones escalan.
4. Usa `-benchmem` y `-cpuprofile` para ver dónde se va el tiempo de la versión con channels.
