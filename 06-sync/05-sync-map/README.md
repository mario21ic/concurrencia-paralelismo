# Ejercicio 5: `sync.Map`

## Teoría

`sync.Map` es un mapa concurrente seguro sin lock explícito. **No** es un reemplazo general de `map` + `Mutex`; la documentación oficial lo recomienda solo para dos patrones:

1. Cada clave se **escribe una vez** y se **lee muchas veces** (cachés que solo crecen).
2. Varias goroutines leen, escriben y sobrescriben **conjuntos de claves disjuntos**.

En Go 1.24 la implementación pasó a un *concurrent hash-trie* (mejor en escrituras y sin la copia de snapshots anterior), pero el consejo se mantiene.

### API

| Método | Descripción |
|--|--|
| `Load(k) (v, ok)` | lee |
| `Store(k, v)` | escribe |
| `LoadOrStore(k, v) (actual, loaded)` | si existe, devuelve el existente; si no, guarda `v` (atómico) |
| `LoadAndDelete(k)` | borra y devuelve el valor |
| `Delete(k)` | borra |
| `Swap(k, v) (prev, loaded)` | reemplaza y devuelve el anterior |
| `CompareAndSwap(k, old, new)` / `CompareAndDelete(k, old)` | operaciones condicionales |
| `Range(f)` | recorre; `f` devuelve `false` para parar; **no** es un snapshot consistente |
| `Clear()` | vacía (Go 1.23+) |

### Desventajas

- **Sin tipos:** claves y valores son `any`; hay que hacer aserciones y se pierde la seguridad de tipos. (Existen envoltorios genéricos de terceros.)
- No tiene `len`. Contar implica `Range`.
- Para estado con invariantes entre varias claves, `map` + `Mutex` es más claro: `sync.Map` solo es atómico por operación (ver ejercicio 7).
- No se debe copiar tras el primer uso.

## Qué hace el programa

Recorre la API (`Load`, `LoadOrStore`, `Swap`, `CompareAndSwap`, `LoadAndDelete`, `Range`, `Clear`) y un caso de "guardar una vez" con 20 goroutines.

```bash
go run .
go test -bench . -benchmem -run xxx
```

## Resultado de referencia

```
LoadOrStore c (nuevo): 3 false
LoadOrStore c (existente): 3 true
CompareAndSwap a 10->11: true
CompareAndSwap a 10->12: false (ya no vale 10)
LoadAndDelete b: 2 true
Range: [a=11 c=3]
tras Clear, elementos: 0
20 goroutines, el valor se guardó una sola vez: true
```

### Benchmarks (8 núcleos, `RunParallel`)

```
BenchmarkLecturaIntensa_SyncMap-8       3.6 ns/op   0 B/op
BenchmarkLecturaIntensa_RWMutexMap-8   74.4 ns/op   0 B/op     <- sync.Map ~20x más rápido
BenchmarkMixto_SyncMap-8               64.9 ns/op  28 B/op
BenchmarkMixto_RWMutexMap-8            73.6 ns/op   0 B/op     <- diferencia pequeña
```

## Conclusiones

- En **lectura intensa con claves estables**, `sync.Map` es muy superior: las lecturas no tocan ningún lock ni contador compartido.
- En un caso **mixto** (50 % escrituras, 16 claves) la ventaja casi desaparece y además asigna memoria (28 B/op).
- Elige `sync.Map` por el patrón de acceso, no por defecto.

## Ejercicios propuestos

1. Añade un benchmark con claves disjuntas por goroutine y compara.
2. Escribe un envoltorio genérico `Mapa[K comparable, V any]` sobre `sync.Map`.
3. Implementa un "cache único por clave" con `LoadOrStore` y comprueba que, aun así, la función costosa puede ejecutarse varias veces (idea para `singleflight`, módulo 10).
