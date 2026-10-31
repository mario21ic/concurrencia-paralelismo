# Ejercicio 7: skip list concurrente

## Teoría

Un **conjunto/mapa ordenado** concurrente (búsqueda, inserción, borrado, recorrido en orden). Un árbol equilibrado (rojo-negro, AVL) es difícil de hacer concurrente porque el reequilibrado toca muchos nodos a la vez. La **skip list** (Pugh, 1990) evita ese problema:

```
nivel 3:  cabeza ───────────────────────► 42 ────────────────► cola
nivel 2:  cabeza ──────────► 17 ────────► 42 ───────► 58 ────► cola
nivel 1:  cabeza ──► 8 ────► 17 ──► 25 ─► 42 ──► 50 ► 58 ────► cola
nivel 0:  cabeza ► 3► 8► 12► 17► 25► 30► 42► 50► 58► 77 ─────► cola
```

- Es una **lista enlazada ordenada** (nivel 0) con **atajos** en niveles superiores. Cada nodo se promociona al nivel `i+1` con probabilidad `1/2`, así que el nº esperado de nodos por nivel se reduce a la mitad.
- Búsqueda, inserción y borrado son **O(log n) esperado**, sin reequilibrado: **insertar solo cambia los punteros de unos pocos vecinos**, y eso es lo que la hace amable con la concurrencia.
- Es la estructura de `ConcurrentSkipListMap` (Java), de la memtable de LevelDB/RocksDB y de los conjuntos ordenados de Redis.

### La skip list "perezosa" (*lazy*) de Herlihy–Lev–Luchangco–Shavit

Un diseño **con cerrojos finos** pero con lecturas **sin espera**:

| Operación | Cómo |
|--|--|
| **`Contains`** | recorre **sin tomar ningún cerrojo**; el nodo existe si `enlazado && !marcado` |
| **`Add`** | busca predecesores/sucesores en todos los niveles **sin cerrojos**; luego cierra solo los predecesores (de abajo arriba), **valida** que nada cambió y enlaza el nodo; al acabar marca `enlazado` (punto de linealización) |
| **`Remove`** | **borrado lógico**: marca el nodo (`marcado = true`, linealización); luego cierra los predecesores, valida y lo **desenlaza físicamente** |

Dos banderas por nodo hacen posible la lectura sin cerrojos:

- `enlazado`: el nodo ya es visible en **todos** sus niveles (un `Add` a medias no cuenta).
- `marcado`: el nodo está **lógicamente borrado** aunque aún esté enlazado.

El patrón general es **búsqueda optimista + cerrar + validar + reintentar**: se hace el trabajo caro sin cerrojos y solo se cierra al final, comprobando que el vecindario sigue igual. Si no, se suelta todo y se repite.

### Qué simplifica Go

En C/C++ un nodo desenlazado no se puede liberar mientras otro hilo pueda estar leyéndolo: hacen falta *hazard pointers* o reclamación por épocas. En Go **el recolector de basura se ocupa**: un lector que aún tenga el puntero lo mantiene vivo. Tampoco hay el problema **ABA** de los punteros reciclados (ejercicio 8): un nodo no se reutiliza mientras alguien lo referencia.

### Comparación honesta

| Opción | Pros | Contras |
|--|--|--|
| **Slice ordenado + `RWMutex`** | trivial; muy compacto; rápido en 1 hilo | inserción/borrado O(n); un único cerrojo ⇒ **no escala** |
| **Skip list perezosa** | lecturas sin cerrojo; escribe con cerrojos locales; escala | más memoria (punteros por nivel); más complejidad; iteración solo débilmente consistente |
| Skip list *lock-free* (Fraser, Sundell–Tsigas) | sin cerrojos | muy compleja; borrado con punteros marcados (en Go no hay bits de marca en punteros) |
| Árbol + `Mutex` | simple | no escala |

## Qué hace el programa

- `skiplist.go`: `Lista[K cmp.Ordered]` con `Add`, `Remove`, `Contains` y `All()` (iterador `iter.Seq`).
- `baseline.go`: `SliceMutex`, la alternativa simple.
- `main.go`: 2 M de operaciones sobre 40 000 claves (mitad llena), con 1, 2, 4 y 8 goroutines y dos mezclas.
- Tests (con `-race`): contra un modelo `map` en secuencial; **concurrente con modelo por goroutine** (cada una es dueña de sus claves pero comparten la estructura); **competencia por las mismas claves** (altas−bajas ∈ {0,1} y coherente con `Contains`); un único ganador al insertar una clave a la vez; orden y ausencia de duplicados al final.

```bash
go run .
go test -race -count=3 -v .
```

## Resultados de referencia (Apple M1, 8 núcleos; ns por operación, reloj / operaciones)

```
90 % Contains, 5 % Add, 5 % Remove:        1       2       4       8   goroutines
  slice ordenado + RWMutex             134.2   180.6   198.6   200.5
  skip list perezosa                   187.4    97.8    59.4    49.0

50 % Contains, 25 % Add, 25 % Remove:      1       2       4       8
  slice ordenado + RWMutex             385.3   477.1   491.6   484.2
  skip list perezosa                   236.3   119.2    70.2    72.3
```

## Conclusiones

- **Con 1 goroutine el slice gana en lecturas** (134 frente a 187 ns): menos indirecciones y mejor localidad de caché. La skip list paga su estructura de punteros.
- **El slice con `RWMutex` empeora al añadir goroutines** (134 → 200 ns): el cerrojo único serializa y su contador de lectores rebota entre núcleos. Con 50 % de escrituras pasa de 385 a ≈ 485 ns a partir de 2 hilos: **no escala nada**.
- **La skip list escala:** con 8 goroutines baja de 187 a 49 ns en la mezcla de lectura (**3.8×**) y de 236 a 72 ns en la de escritura (**3.3×**). Con 8 hilos es **4.1×** (90 % lecturas) y **6.7×** (50 %) más rápida que el slice.
- **El coste de las escrituras en el slice crece con n** (memmove de miles de elementos); en la skip list es O(log n).
- La corrección se verifica con **modelos** (un `map` de referencia) y **balances** (altas−bajas); los tests concurrentes con `-race` encontrarían carreras de datos, y los de balance, pérdidas de actualizaciones.

## Ejercicios propuestos

1. Añade valores (`Mapa[K, V]`) y una operación `Seek(k)` que devuelva el iterador desde la primera clave `>= k`.
2. Añade `Len()` aproximado con un contador fragmentado (módulo 13).
3. Implementa un `RangeQuery(a, b)` y razona qué garantía de consistencia da.
4. Cambia la probabilidad de ascenso de 1/2 a 1/4 y mide altura media y rendimiento.
