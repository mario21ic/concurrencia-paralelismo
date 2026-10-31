# Ejercicio 6: hash map con sharding

## Teoría

Un `map` de Go **no es seguro para uso concurrente** (un acceso con escritura concurrente provoca `fatal error: concurrent map writes`). La solución evidente es un `sync.RWMutex` alrededor, pero con muchos núcleos ese **único cerrojo** se convierte en el cuello de botella: todas las operaciones se serializan y, además, el contador de lectores de `RWMutex` es un atómico compartido (módulo 13: la línea de caché rebota).

### Fragmentación (*sharding*)

Se reparte el espacio de claves entre **N mapas independientes**, cada uno con su propio cerrojo:

```
shard = hash(clave) & (N-1)        // N potencia de 2: máscara en vez de módulo
```

Dos operaciones sobre claves de **fragmentos distintos** no se bloquean entre sí. Es el mismo principio que el contador fragmentado del módulo 13, y el de `ConcurrentHashMap` (Java) o `DashMap` (Rust).

Detalles de diseño que importan:

| Decisión | Motivo |
|--|--|
| **Hash con semilla** (`hash/maphash`, `maphash.Comparable`) | reparto uniforme y resistente a claves malintencionadas; funciona con cualquier `comparable` |
| **Relleno** (`cpu.CacheLinePad`) entre fragmentos | evita *false sharing* entre cerrojos vecinos |
| **`Update(k, fn)`** atómico | `Load` + `Store` por separado **no** es atómico (se pierden actualizaciones); la operación leer-modificar-escribir debe ocurrir bajo el cerrojo del fragmento |
| **`Range` sobre copias por fragmento** | permite llamar al mapa desde el callback sin interbloqueo; la visión es consistente **por fragmento**, no global |
| **`Len` suma fragmento a fragmento** | no es una instantánea: otra goroutine puede cambiar entre medias |

### ¿Cuántos fragmentos?

Más fragmentos ⇒ menos contención pero más memoria y peor localidad. Como regla: de **4× a 16×** el número de núcleos. Más allá hay rendimientos decrecientes.

### `sync.Map` frente a un mapa fragmentado

`sync.Map` (desde Go 1.24 basada en un *hash-trie* concurrente) está pensada para dos patrones: claves que se **escriben una vez y se leen muchas veces**, o goroutines que operan sobre **conjuntos de claves disjuntos**. Es de tipo `any` (sin genéricos, con conversiones) y no tiene operaciones de lectura-modificación-escritura generales. Un mapa fragmentado con un `RWMutex` por fragmento es más predecible y tipado.

### Límite de la técnica: la clave caliente

La fragmentación reparte **claves**, no **operaciones**. Si el 30 % del tráfico va a una sola clave, ese fragmento sigue siendo un cerrojo único. Soluciones: replicar el dato (réplicas de lectura), cachear en cada goroutine, o *combining* (agrupar actualizaciones antes de aplicarlas).

## Qué hace el programa

Compara `RWMutex` global, `sync.Map` y `Fragmentado` (4, 16, 64, 256 fragmentos) con 8 goroutines, 65 536 claves y 8 M de operaciones, en tres cargas: 95 % lecturas, 50 % lecturas, y 50 % lecturas con 30 % del tráfico en una clave. Los tests (con `-race`) comprueban que `Update` es atómico y `Load`+`Store` no, que `LoadOrStore` tiene un único ganador, que `Range` tolera modificaciones y que el reparto de claves es uniforme.

```bash
go run .
go test -race -v .
```

## Resultados de referencia (Apple M1, 8 núcleos; ns por operación, tiempo de reloj / operaciones totales)

```
95 % lecturas, claves uniformes:
  RWMutex global            65.4      sync.Map              16.3
  Fragmentado (4)           46.3      Fragmentado (16)      30.1
  Fragmentado (64)          21.7      Fragmentado (256)     14.6

50 % lecturas, claves uniformes:
  RWMutex global            68.3      sync.Map              37.6
  Fragmentado (4)           55.4      Fragmentado (16)      38.3
  Fragmentado (64)          23.6      Fragmentado (256)     21.4

50 % lecturas, 30 % de las ops en UNA clave caliente:
  RWMutex global            74.6      sync.Map              56.9
  Fragmentado (4)           64.3      Fragmentado (16)      57.0
  Fragmentado (64)          53.7      Fragmentado (256)     53.8
```

## Conclusiones

- **El cerrojo global es 4.5× más lento** que 256 fragmentos con 95 % de lecturas (65 → 14.6 ns) y **3.2×** con 50 %: la fragmentación convierte un cuello de botella en N cuellos independientes.
- **Los rendimientos decrecen:** 4 → 16 → 64 → 256 fragmentos da 46 → 30 → 22 → 15 ns; en la carga de 50 % se estabiliza en 64 (23.6 → 21.4).
- **`sync.Map` empata en lecturas con 256 fragmentos** (16.3 frente a 14.6 ns), que es su caso de uso; con muchas escrituras (37.6 ns) lo supera un mapa fragmentado de 64+ fragmentos (23.6 ns).
- **La clave caliente aplana todo:** con 30 % de las operaciones en una clave, 256 fragmentos (53.8 ns) apenas mejoran a 64 (53.7) y el global solo es 1.4× peor. La fragmentación **no ayuda** cuando el problema es una única clave.
- El tipo genérico `Fragmentado[K, V]` evita las conversiones de `sync.Map` y permite añadir operaciones atómicas (`Update`, `LoadOrStore`) que `sync.Map` no ofrece de forma general.

## Ejercicios propuestos

1. Añade `Snapshot()` que bloquee **todos** los fragmentos (en orden, para no interbloquear) y devuelva una instantánea global; mide su coste.
2. Compara el reparto con `hash/maphash` frente a `hash/fnv` y claves secuenciales.
3. Implementa **redimensionado** de fragmentos (¿cómo se cambia N sin parar el mundo?).
4. Aborda la clave caliente: añade una caché por goroutine para lecturas de claves calientes con invalidación.
