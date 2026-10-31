# Módulo 7. Paquete `sync/atomic`

**Objetivo:** usar operaciones atómicas con criterio: saber qué garantizan, qué *no* garantizan y cuándo un mutex o un channel son la mejor opción.

## Ejercicios

| # | Carpeta | Tema |
|---|---------|------|
| 1 | [01-contadores](01-contadores) | `Add`, `Load`, `Store`, `Swap`, `CompareAndSwap`; tipos atómicos |
| 2 | [02-cas-loops](02-cas-loops) | Bucles CAS: máximo concurrente, retirada con fondos, spinlock |
| 3 | [03-pointer-value](03-pointer-value) | `atomic.Pointer[T]`, `atomic.Value`, configuración recargable |
| 4 | [04-and-or](04-and-or) | `And`/`Or` atómicos (Go 1.23+) con máscaras de bits |
| 5 | [05-benchmark-contador](05-benchmark-contador) | **Ejercicio:** contador con `Mutex` vs `atomic` vs channel |
| 6 | [06-pila-lock-free](06-pila-lock-free) | Estructura lock-free: pila de Treiber |
| 7 | [07-miniproyecto-cache-ttl](07-miniproyecto-cache-ttl) | **Mini-proyecto 2:** caché con TTL y expiración por `context` |

```bash
cd 07-atomic/01-contadores && go run .
cd 07-atomic/05-benchmark-contador && go test -bench . -benchmem
```

> Mediciones: Mac M1 de 8 núcleos, Go 1.26.5.

## Resumen teórico

Una operación atómica se ejecuta de forma **indivisible** y, además, establece **happens-before** (el Go Memory Model las trata como secuencialmente consistentes): si un `Load` observa el valor de un `Store`, todo lo que ocurrió antes de ese `Store` es visible tras el `Load`.

### API

| Operación | Devuelve |
|--|--|
| `Load()` | valor actual |
| `Store(v)` | nada |
| `Add(d)` | el valor **nuevo** (solo enteros) |
| `Swap(v)` | el valor **anterior** |
| `CompareAndSwap(old, new)` | `bool`: ¿se cambió? (solo si el valor era `old`) |
| `And(m)` / `Or(m)` (1.23+) | el valor **anterior** (solo enteros sin signo/con signo según tipo) |

### Tipos (preferibles a las funciones antiguas `atomic.AddInt64(&x, 1)`)

`atomic.Int32`, `Int64`, `Uint32`, `Uint64`, `Uintptr`, `Bool`, `Pointer[T]`, `Value`.

Ventajas de los tipos: no se pueden usar por accidente sin atomic, garantizan la alineación a 64 bits en plataformas de 32 bits y `go vet` impide copiarlos.

### Lo que atomic NO da

1. **Atomicidad de secuencias.** `x.Load()` seguido de `x.Store(v+1)` **no** es atómico. Dos atómicos distintos tampoco forman una unidad (ejercicio 3). Para varios campos: publica un único puntero a una estructura inmutable, o usa un mutex.
2. **Protección de invariantes entre varios datos.**
3. **Espera.** No hay "bloquear hasta que cambie": solo `Load` en bucle (espera activa), mala idea.

### Cuándo usar cada herramienta

| Necesidad | Herramienta |
|--|--|
| Contador, bandera, estadística independiente | `atomic` |
| Publicar un snapshot inmutable de configuración | `atomic.Pointer[T]` |
| Estado con invariantes entre varios campos | `Mutex` |
| Transferir propiedad, coordinar, pipelines | channel |
| Estructura lock-free propia | casi nunca; solo con medidas que lo justifiquen |

> **Regla:** empieza con un mutex. Pasa a atomic cuando un perfil demuestre que el lock es el cuello de botella y el estado sea un único valor.

### Lock-free no significa más rápido

*Lock-free* garantiza que **alguna** goroutine progresa siempre (sin interbloqueos), no que sea más veloz. Bajo contención los bucles CAS reintentan y consumen CPU. Es más difícil de escribir y de razonar (ABA, orden de memoria).

### El problema ABA

Un CAS solo comprueba "¿sigue valiendo `A`?", no "¿no ha cambiado nunca?". Si el valor pasó de `A` a `B` y volvió a `A`, el CAS tiene éxito aunque el estado interno sea distinto. En Go el GC evita el caso clásico de **reutilización de direcciones** mientras tengas un puntero al nodo, pero ABA sigue siendo posible si **reutilizas nodos** (p. ej. con `sync.Pool` o listas libres).

## Preguntas de repaso

1. ¿Por qué `x.Store(x.Load() + 1)` no es un incremento atómico?
2. ¿Qué devuelve `Add`? ¿Y `Swap`? ¿Y `Or`?
3. ¿Por qué dos `atomic.Int64` no pueden proteger la invariante `b == 2*a`?
4. ¿Cómo se actualiza de forma consistente una configuración con varios campos sin mutex?
5. ¿Qué es un bucle CAS y cuándo reintenta?
6. ¿Por qué lock-free no implica mayor rendimiento?
7. ¿Qué es ABA y por qué el GC lo mitiga en Go?
8. ¿Por qué `atomic.Value.Store(nil)` y almacenar tipos distintos provocan `panic`?

## Lecturas

- Documentación de [`sync/atomic`](https://pkg.go.dev/sync/atomic)
- [The Go Memory Model](https://go.dev/ref/mem) — sección sobre atomics
- R. K. Treiber, *Systems Programming: Coping with Parallelism* (1986) — la pila lock-free
- *The Art of Multiprocessor Programming* — Herlihy & Shavit (capítulos de CAS y estructuras lock-free)
