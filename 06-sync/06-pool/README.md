# Ejercicio 6: `sync.Pool`

## Teoría

`sync.Pool` es una **caché de objetos temporales** reutilizables, pensada para aliviar la presión sobre el recolector de basura cuando se crean y descartan muchos objetos del mismo tipo (buffers, codificadores, estructuras de trabajo).

```go
var pool = sync.Pool{New: func() any { return new(bytes.Buffer) }}

buf := pool.Get().(*bytes.Buffer)
buf.Reset()
defer pool.Put(buf)
```

### Cómo funciona (a grandes rasgos)

- Cada P tiene su propia cola local, de modo que `Get`/`Put` casi no compiten entre sí.
- Si el pool local está vacío, intenta robar de otro P; si no hay nada, llama a `New`.
- En cada ciclo de **GC** el contenido pasa a una caché "víctima" y, en el siguiente, se descarta.

### Reglas

1. **Es una caché, no un almacén.** Cualquier objeto puede desaparecer en cualquier momento. Nunca lo uses para guardar estado que no puedas recrear.
2. **Resetea** el objeto antes de usarlo (o al devolverlo): `buf.Reset()`. Un objeto sucio contamina el siguiente uso (y puede filtrar datos entre peticiones).
3. **No uses el objeto tras `Put`**: otra goroutine puede tomarlo.
4. No pongas objetos de **tamaño muy variable** sin límite (un buffer de 100 MB devuelto al pool lo retiene): descarta los que excedan un máximo.
5. Guarda **punteros** (`*bytes.Buffer`, `*[]byte`). Meter un slice directamente en `any` asigna memoria en cada `Put`.
6. No es adecuado para recursos con ciclo de vida (conexiones, archivos): usa un pool propio.
7. Medir: si los objetos son pequeños y baratos, el pool puede no compensar.

## Qué hace el programa

1. 1000 usos secuenciales: se crea 1 objeto.
2. 8 goroutines × 10 000 usos: se crean ~tantos como goroutines concurrentes, no 80 000.
3. Tras dos `runtime.GC()`, el objeto guardado desaparece.

```bash
go run .
go test -bench . -benchmem -run xxx
```

## Resultado de referencia

```
secuencial, 1000 usos -> objetos creados: 1
8 goroutines, 80000 usos -> objetos creados: 7
tras 2 GC, ¿Get creó uno nuevo? true (no confíes en que los objetos persistan)
```

### Benchmarks (8 núcleos, `RunParallel`)

```
BenchmarkSinPool         88 ns/op     136 B/op   4 allocs/op
BenchmarkConPool         56 ns/op      24 B/op   2 allocs/op      <- objetos pequeños: mejora moderada

BenchmarkGrandeSinPool  5272 ns/op  65536 B/op   1 allocs/op
BenchmarkGrandeConPool     2 ns/op      0 B/op   0 allocs/op      <- buffers de 64 KB: gran mejora
```

> El benchmark "grande" es favorable al pool a propósito: la versión con pool apenas hace trabajo (solo toca el primer byte), por lo que el 2 ns refleja el coste de `Get`/`Put`. Úsalo como orden de magnitud del *ahorro de asignación*, no como el rendimiento de un caso real.

## Conclusiones

- Con objetos pequeños, el pool recorta tiempo (~36 %) y asignaciones a la mitad; el beneficio es moderado.
- Con objetos grandes, evita 64 KB de asignación por operación: menos trabajo para el GC.
- El número de objetos vivos es proporcional a la **concurrencia**, no a las operaciones.

## Ejercicios propuestos

1. Cambia `buf.Reset()` por nada y comprueba que los resultados se contaminan (usa `buf.WriteString` sin reset).
2. Añade un máximo de capacidad (`if buf.Cap() > 64<<10 { return }`) antes de `Put`.
3. Mide con `GODEBUG=gctrace=1` cuántos GC ocurren con y sin pool.
4. Busca usos de `sync.Pool` en la biblioteca estándar (`fmt`, `encoding/json`).
