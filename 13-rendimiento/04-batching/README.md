# Ejercicio 4: batching — amortizar costes fijos

## Teoría

Casi toda operación de sincronización tiene un **coste fijo** independiente de cuántos datos mueve:

| Operación | Coste típico |
|--|--|
| enviar por un channel | ~50-100 ns |
| tomar un mutex disputado | ~20-100+ ns |
| una llamada a red / syscall | µs - ms |
| asignar un objeto | ~20-100 ns |

Si el trabajo útil por elemento es pequeño, ese coste domina. **Agrupar N elementos en un lote** reparte el coste fijo entre N: `coste/elemento ≈ trabajo + fijo/N`.

### Aplicaciones

- **Channels:** enviar `[]T` (lotes) en vez de `T`.
- **Locks:** acumular localmente y tomar el lock **una vez por lote**.
- **E/S:** escrituras en lotes (`bufio.Writer`), `INSERT` múltiples, `WriteBatch`.
- **Asignaciones:** reutilizar el lote (pool o canal de devolución) para tener **cero asignaciones** en régimen.
- **Preasignar** capacidad (`make([]T, 0, n)`, `make(map, n)`, `strings.Builder.Grow`).

### El precio: latencia y memoria

El primer elemento de un lote espera a que se llene. Soluciones: lote **por tamaño o por tiempo** (lo que ocurra antes: el `Batch` del módulo 8) y tamaños moderados. Cuanto mayor el lote, menos mejora marginal: **hay un punto de rendimientos decrecientes**.

## Qué hace el programa

**Parte 1:** un productor envía 8 M enteros a un consumidor por un channel, agrupados en lotes de 1 a 1024; con y sin reciclar los lotes (por otro channel).
**Parte 2:** 8 workers acumulan en un total protegido por un mutex, tomando el lock cada N elementos.
Además `prealoc_test.go` mide el efecto de preasignar.

```bash
go run .
go test -bench . -benchmem -run xxx
```

## Resultados de referencia

**Parte 1 (ns por elemento):**

```
    lote    ns/elemento     vs. lote 1     reciclando     vs. lote 1
       1           50.2           1.0x           81.0           1.0x
       4           14.8           3.4x           19.5           4.1x
      16            4.7          10.6x            8.9           9.1x
      64            2.8          18.2x            2.9          28.1x
     256            1.5          32.4x            1.3          61.2x
    1024            1.7          29.8x            1.1          70.9x
```

**Parte 2 (8 workers, mutex):**

```
    lote    ns/elemento     vs. lote 1
       1          86.90           1.0x
       4          20.60           4.2x
      16           5.82          14.9x
      64           1.37          63.5x
    1024           0.18         481.4x
```

**Preasignar** (1000 elementos):

```
AppendSinPreasignar        4110 ns   25208 B  12 allocs
AppendPreasignado          2893 ns    8192 B   1 allocs      (1.4×)
MapSinPreasignar          37340 ns   74456 B  22 allocs
MapPreasignado            11694 ns   36992 B   6 allocs      (3.2×)
StringConcatenar           3532 ns   15992 B  99 allocs
StringBuilderPreasignado    255 ns     320 B   1 allocs     (13.8×)
```

## Conclusiones

- Con trabajo trivial por elemento, el channel cuesta **50 ns/elemento** y con lotes de 256 baja a **1.5 ns**: ~32× más rápido. Reciclando los lotes, **~61-71×**.
- **Los rendimientos decrecen:** de 256 a 1024 apenas mejora (1.5 → 1.7 ns).
- **Reciclar solo compensa con lotes grandes:** con lote de 1 es *peor* (81 vs 50 ns) por el coste de la `select` de devolución. Cada técnica tiene su punto de equilibrio.
- **La cifra de 481× del mutex es un máximo teórico:** el "trabajo" por elemento es un `local++` (casi 0); con trabajo real, el factor sería mucho menor, porque el lock deja de ser el coste dominante. El mensaje no es "481×" sino "el lock por elemento era casi todo el coste".
- **Preasignar** cuesta una línea y da 1.4× (slices), 3.2× (mapas) y hasta 13.8× (cadenas). Reduce asignaciones de 99 a 1 en el caso de las cadenas.
- Todo batching tiene el coste de latencia: pensar en el tamaño máximo y en un temporizador.

## Ejercicios propuestos

1. Añade un temporizador para emitir un lote parcial cada 5 ms y mide el efecto en la latencia del primer elemento.
2. Repite la parte 1 con 4 productores y un consumidor.
3. Sustituye el canal de devolución por `sync.Pool` y compara.
4. Cambia el trabajo por elemento por un cálculo de ~200 ns: ¿cuánto queda de la mejora?
