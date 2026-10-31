# Ejercicio 5: benchmark de un contador

## Enunciado

Comparar el coste de incrementar un contador compartido con: `Mutex`, `atomic`, un channel (modelo CSP) y una estrategia que evita la contención (agregación local).

## Teoría

Los benchmarks usan `testing.B`:

- `b.RunParallel(func(pb *testing.PB) { for pb.Next() { ... } })` ejecuta el cuerpo desde `GOMAXPROCS` goroutines y reparte las `b.N` iteraciones. **ns/op** es el tiempo de reloj por operación del conjunto, no por goroutine.
- `for b.Loop() { ... }` (Go 1.24+) es la forma moderna para benchmarks secuenciales.
- `-benchmem` añade bytes y asignaciones por operación.

Conceptos que explican los resultados:

- **Sin contención** una instrucción atómica o un lock son baratos: no hay nadie con quien competir.
- **Con contención**, todos los núcleos pelean por la misma **línea de caché**; esa línea viaja de núcleo en núcleo en cada operación. Es el cuello de botella del hardware (se ve más en el módulo 13: *false sharing*).
- El `Mutex` además puede **aparcar** goroutines (cambios de contexto), lo que lo encarece bajo contención.
- Un channel serializa en una goroutine consumidora: cada envío con buffer es relativamente barato mientras el buffer no se llena.
- La mejor sincronización es la que **no hace falta**: contar localmente y combinar una vez.

## Cómo ejecutarlo

```bash
go test -bench . -benchmem -run xxx
go test -bench . -cpu 1,2,4,8       # escalabilidad con el número de núcleos
```

## Resultados de referencia (8 núcleos)

```
BenchmarkMutex-8                  91.2 ns/op
BenchmarkAtomic-8                 33.7 ns/op
BenchmarkChannel-8                72.5 ns/op
BenchmarkAgregacionLocal-8       0.199 ns/op
BenchmarkSecuencialSinSync-8      2.13 ns/op
BenchmarkMutexSinContencion-8     13.9 ns/op
BenchmarkAtomicSinContencion-8    7.11 ns/op
```

## Conclusiones

- **Con contención**, `atomic` es ~2.7× más rápido que `Mutex` en este caso (33.7 vs 91.2 ns).
- **Sin contención** la diferencia es de ~2× (7.1 vs 13.9 ns), y ambos son mucho más caros que no sincronizar (2.1 ns).
- **Sorpresa:** el channel con buffer (72.5 ns) fue **más rápido que el mutex** bajo contención. Aquí los productores solo escriben en una cola y un único consumidor hace el trabajo; el mutex en cambio provoca parada y reactivación de goroutines. No generalices: con buffer pequeño o muchos consumidores el resultado cambia.
- **La agregación local** es ~170× más rápida que `atomic` (0.2 ns/op medido por reloj sobre 8 núcleos): eliminar la contención gana a abaratarla. El benchmark es favorable por construcción (no hay trabajo real), pero la idea es general: acumula localmente y combina al final.
- Ningún resultado sustituye a medir **tu** carga real: una sección crítica con lógica de varios pasos no es un `n++`.

## Ejercicios propuestos

1. Ejecuta con `-cpu 1,2,4,8` y dibuja la escalabilidad de cada versión.
2. Añade un contador **fragmentado** (sharded): N atómicos en un array con relleno (*padding*) para evitar *false sharing*; cada goroutine elige el suyo.
3. Cambia el buffer del channel a 0 y a 1 y mide.
4. Añade `RWMutex` y justifica por qué no tiene sentido para un contador.
