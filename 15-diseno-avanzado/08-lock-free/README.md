# Ejercicio 8: lock-free — ABA, bucles CAS y límites

## Teoría

### Garantías de progreso

| Garantía | Significado | Ejemplo |
|--|--|--|
| **Con bloqueo** (*blocking*) | si el hilo que tiene el cerrojo se detiene (preemption, fallo de página, GC), **todos** esperan | `sync.Mutex` |
| **Sin obstáculos** (*obstruction-free*) | un hilo progresa si corre solo | algunos esquemas de memoria transaccional |
| **Lock-free** | **algún** hilo siempre completa su operación; un hilo concreto puede fracasar indefinidamente | pila de Treiber, cola de Michael–Scott, bucle CAS |
| **Wait-free** | **todo** hilo completa su operación en un nº acotado de pasos | `atomic.Add` en hardware, algunos contadores |

Lock-free **no** significa "más rápido": significa que **un hilo parado no puede paralizar a los demás** (importa con prioridades, señales, preemption). Con mucha contención puede ser *más lento* que un mutex.

### El bucle CAS

```go
for {
    viejo := x.Load()
    nuevo := f(viejo)
    if x.CompareAndSwap(viejo, nuevo) { return }
}
```

Optimista: calcula sin bloquear y publica solo si nadie cambió `x` entre medias. Si falla, **repite el cálculo** (trabajo desperdiciado). El fallo de un CAS implica que **otro hilo progresó**: ahí está el "lock-free". Requisitos: `f` pura y sin efectos colaterales (se puede ejecutar varias veces).

### El problema ABA

El CAS compara **valores**, no **historia**. Si entre la lectura y el CAS la posición pasó de `A` a `B` y volvió a `A`, el CAS cree que no pasó nada. En una pila:

```
pila = A → B → C
T1 lee cabeza=A, siguiente=B             ← la interrumpen
T2: pop A, pop B, push A' (reutiliza el hueco de A)     pila: A' → C
T1: CAS(cabeza: A → B) TRIUNFA           pila: B → C     ← B ya había salido; A' se perdió
```

| Solución | Idea | Coste |
|--|--|--|
| **Etiqueta de versión** (*tagged pointer*) | `cabeza = (versión << 32) | índice`; cada CAS incrementa la versión | un CAS de doble anchura o empaquetado |
| **GC** (Go, Java) | un nodo no se reutiliza mientras alguien lo apunta | gratis, pero solo con punteros |
| **Hazard pointers / épocas (RCU)** | no se libera un nodo hasta que nadie pueda estar leyéndolo | complejidad |
| LL/SC (ARM, POWER, RISC-V) | `SC` falla si hubo **cualquier** escritura | solo en esas arquitecturas |

**En Go con punteros el ABA no ocurre**: es el motivo de que `PilaTreiber` sea tan sencilla. Reaparece cuando los "punteros" son **índices en un array**, en pools de memoria propios o con `unsafe`.

### Estructuras del ejercicio

- **Pila de Treiber**: `cabeza` atómica; `Push` enlaza el nodo y hace CAS; `Pop` hace CAS al siguiente.
- **Cola de Michael–Scott**: lista con nodo centinela, `cabeza` y `cola` atómicas. Clave: **ayuda cooperativa**: si un hilo ve que `cola` va atrasada (otro enlazó pero aún no la avanzó), la **avanza él**; así ningún hilo lento bloquea a los demás.
- **`PilaIdx`**: pila sobre un array con índices reutilizables, con y sin etiqueta de versión, para **demostrar el ABA de forma determinista** separando `Pop` en `Leer` + `Confirmar`.

### Límites de la técnica

1. **Más difícil de acertar:** razonar sobre todos los entrelazados (y el modelo de memoria). Errores sutiles que ni `-race` ve.
2. **No siempre es más rápido:** el CAS fallido repite trabajo y la línea de caché sigue rebotando; **un contador compartido sigue siendo un punto caliente** aunque sea atómico (módulo 13).
3. **Asigna un nodo por elemento** (presión sobre el GC); un buffer circular preasignado (ejercicio 5) evita eso.
4. **Sin garantía de equidad:** una goroutine puede reintentar muchas veces (lock-free ≠ wait-free).
5. **Operaciones compuestas son difíciles:** actualizar *dos* estructuras atómicamente exige un cerrojo, una transacción o rediseñar.
6. **Espera activa:** las estructuras sin cerrojos no duermen a quien no puede avanzar (una pila vacía hay que sondearla).

> **Regla:** usa `sync.Mutex`/channels salvo que *mida* que son el cuello de botella y entiendas el modelo de memoria. Prefiere `atomic.Add`/`atomic.Pointer` simples a estructuras lock-free de tu cosecha.

## Qué hace el programa

| Archivo | Contenido |
|--|--|
| `estructuras.go` | `PilaTreiber`, `PilaMutex`, `ColaMS` (Michael–Scott), `ColaMutex` |
| `aba.go` | `PilaIdx` con/sin etiqueta y `EscenarioABA` determinista |
| `contador.go` | contador con mutex, con bucle CAS (cuenta reintentos) y con `atomic.Add` |
| `main.go` | A) ABA; B) contadores con 1-8 goroutines; C) pilas/colas/channel con 4+4 |

```bash
go run .
go test -race -v .
```

## Resultados de referencia (Apple M1, 8 núcleos, Go 1.26.5)

```
A) sin etiqueta: T1 hace CAS(A→B) → TRIUNFA → pila [B → C]    (corrupta: B resucitado, A' perdido)
   con etiqueta: T1 hace CAS(A→B) → FALLA   → pila [X → C]    (intacta)

B) ns por incremento           mutex        bucle CAS     atomic.Add    reintentos CAS/op (media / máx)
   1 goroutine                 15.1          7.2           7.0           0.00 / 0
   2                           41.9         25.9          18.0           0.43 / 51
   4                           66.4         44.1          24.1           0.63 / 1099
   8                           89.5         79.3          36.5           1.46 / 5082

C) 4 productores + 4 consumidores (ns por elemento = 1 inserción + 1 extracción)
   pila Treiber (lock-free)  205.0     pila con mutex   192.7
   cola Michael-Scott        235.7     cola con mutex   184.1
   chan int (buffer 1024)     38.8
```

## Conclusiones

- **El ABA es real y silencioso:** el CAS "triunfa", no hay pánico ni `race`, y la estructura queda corrupta (`[B → C]` en vez de `[C]`). Una etiqueta de versión de 32 bits lo evita. Los tests reproducen el entrelazado exacto, y el estrés con etiqueta conserva la suma de elementos.
- **Escala mal por contención, aunque sea atómico:** con 8 goroutines el bucle CAS tarda 79 ns frente a 7 ns con 1 (11×); `atomic.Add` 36.5 ns (5×). Todos comparten **una** línea de caché.
- **El CAS pierde trabajo:** con 8 goroutines hay 1.46 reintentos por operación de media y **hasta 5 082 seguidos** para una misma goroutine: **lock-free no es wait-free**. `atomic.Add` no tiene reintentos y es 2.2× más rápido que el bucle CAS con 8.
- **Con 8 goroutines el mutex (89.5 ns) y el bucle CAS (79.3) casi empatan:** el mutex no es el "villano" que se supone cuando la sección crítica es corta.
- **Las estructuras lock-free no ganaron al mutex** (pila 205 frente a 193 ns; cola 236 frente a 184), y **el channel con buffer fue 5× más rápido que todas** (38.8 ns): el runtime duerme a quien no puede avanzar (las versiones lock-free sondean con `Gosched`) y el channel no asigna un nodo por elemento.
- **Moraleja:** lock-free resuelve el problema de *progreso* (un hilo parado no bloquea a otros), no el de *rendimiento*. Para rendimiento: menos contención (módulo 13), lotes, buffers preasignados.

## Ejercicios propuestos

1. Aumenta la etiqueta a 16 bits y busca cuándo vuelve a ocurrir el ABA (desbordamiento de la etiqueta).
2. Añade `PilaIdx.PopN` con un único CAS que saque varios elementos y razona sobre su ABA.
3. Implementa un contador wait-free *fragmentado* (módulo 13) y compáralo con los tres de aquí.
4. Añade a `ColaMS` un `Close` y un `Dequeue` bloqueante usando un channel de señal, y mide el coste.
