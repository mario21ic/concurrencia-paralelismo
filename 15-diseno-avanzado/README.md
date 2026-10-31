# Módulo 15. Diseño avanzado y problemas clásicos

**Objetivo:** pasar de *usar* primitivas a **diseñar** sistemas concurrentes: reconocer y evitar los fallos de progreso (deadlock, livelock, inanición, inversión de prioridad), resolver los problemas clásicos, construir estructuras concurrentes propias sabiendo cuándo **no** hacerlo, y diseñar APIs y servidores que se comporten bien bajo carga.

## Ejercicios

Cada carpeta tiene su propio `README.md` con la teoría, qué hace el programa, resultados de referencia, conclusiones y ejercicios propuestos.

| # | Carpeta | Tema |
|---|---------|------|
| 1 | [01-interbloqueos](01-interbloqueos) | Deadlock, livelock, inanición, inversión de prioridad (herencia y envejecimiento) |
| 2 | [02-filosofos](02-filosofos) | Cena de los filósofos: orden global, mayordomo, asimetría; el deadlock latente |
| 3 | [03-lectores-escritores](03-lectores-escritores) | Preferencia de lectores / escritores / justa; comparación con `sync.RWMutex` |
| 4 | [04-barbero](04-barbero) | Barbero dormilón: cola acotada, pérdidas y latencia con `synctest` |
| 5 | [05-cola-y-ring](05-cola-y-ring) | Cola acotada con `Cond` y `ctx`; ring buffers SPSC y MPMC (Vyukov) |
| 6 | [06-mapa-sharded](06-mapa-sharded) | Hash map fragmentado genérico frente a `RWMutex` y `sync.Map` |
| 7 | [07-skiplist](07-skiplist) | Skip list concurrente perezosa: lecturas sin cerrojo, escala con los núcleos |
| 8 | [08-lock-free](08-lock-free) | Treiber, Michael–Scott, el problema ABA, bucles CAS y límites |
| 9 | [09-apis-concurrentes](09-apis-concurrentes) | Quién lanza y quién detiene la goroutine; concurrencia estructurada |
| 10 | [10-http-backpressure](10-http-backpressure) | `net/http`: sobrecarga, descarte de carga, cola acotada, apagado ordenado |
| 11 | [11-database-sql](11-database-sql) | Pool de `database/sql`, agotamiento, filas sin cerrar, cancelación de transacciones |

```bash
cd 15-diseno-avanzado/01-interbloqueos && go run .
go test -race ./15-diseno-avanzado/...
```

> **Hardware y versión de las mediciones:** Apple M1, 8 núcleos, macOS, Go 1.26.5. Las cifras absolutas cambian con tu máquina; lo reutilizable es el **método** y la **forma** de los resultados.

## Resumen teórico

### 1. Los cuatro fallos de progreso

| Fallo | Qué pasa | Cómo se detecta | Remedio típico |
|--|--|--|--|
| **Deadlock** | ciclo de espera (Coffman: exclusión mutua + mantener y esperar + sin expropiación + espera circular) | volcado de goroutines / `pprof`; el runtime solo si **todas** están dormidas | orden global de recursos; `TryLock` con plazo y *jitter*; no retener uno mientras se pide otro |
| **Livelock** | todos se mueven, nadie avanza (cortesía simétrica) | CPU alta sin progreso | romper la simetría: retardo aleatorio |
| **Starvation** | el sistema progresa, una goroutine no | su espera máxima crece sin tope | justicia (FIFO), envejecimiento |
| **Inversión de prioridad** | la tarea urgente espera a la baja, desplazada por las medias | latencia de la urgente | herencia/techo de prioridad |

**Regla:** un test que pasa no demuestra la ausencia de deadlock: depende de la temporización. Razona sobre el **orden de adquisición**.

### 2. Problemas clásicos como modelos

| Problema | Modela | Lección |
|--|--|--|
| Filósofos | adquirir varios recursos | el orden global es la solución barata y robusta; evitar el deadlock no garantiza justicia |
| Lectores–escritores | lectura masiva, escritura rara | la política de preferencia decide **quién** se inanicia; `sync.RWMutex` ya hace lo correcto |
| Barbero dormilón | cola acotada con rechazo | más sillas ≠ mejor: mueven la pérdida a latencia; la capacidad la da el nº de servidores |

### 3. Estructuras concurrentes propias: cuándo y cómo

| Estructura | Idea | Escala si… | Cuidado |
|--|--|--|--|
| Cola acotada (`Cond`) | mutex + dos condiciones, `ctx` con `context.AfterFunc` | no es su fuerte | un `chan` suele ser mejor |
| Ring SPSC | contadores separados + copia local del ajeno | 1 productor + 1 consumidor | hay que girar; *false sharing* |
| Ring MPMC (Vyukov) | secuencia por celda + CAS | pocos productores | el CAS sobre un contador es un punto caliente |
| Mapa fragmentado | N mapas con su cerrojo | claves bien repartidas | una **clave caliente** lo anula |
| Skip list perezosa | optimista: buscar sin cerrojo, cerrar, validar | lecturas masivas + orden | complejidad; iteración débilmente consistente |
| Lock-free (Treiber, MS) | CAS | tampoco necesariamente | ABA, equidad, asignaciones |

### 4. Lock-free y wait-free

- **Lock-free:** algún hilo progresa siempre; **wait-free:** todos, en pasos acotados. Ninguno promete ser **más rápido**.
- **ABA:** el CAS compara valores, no historia. En Go con punteros lo evita el GC; con índices/pools propios, una **etiqueta de versión**.
- La ventaja real es la **robustez frente a hilos detenidos**, no el rendimiento bruto.

### 5. Diseño de APIs concurrentes

1. **Deja que el llamante lance** (`Run(ctx) error` bloqueante). Si el componente necesita hilos propios, ofrece `Close()` que **espera**.
2. Toda goroutine necesita una **condición de salida** y un **dueño**.
3. Channels: quien envía, cierra; cada envío compite con `ctx.Done()`; un **iterador** es más seguro si no hace falta paralelismo.
4. **Concurrencia estructurada:** ninguna goroutine sobrevive a la función que la lanzó (`errgroup`, `WaitGroup.Go`, `recover` en las hijas).

### 6. Servidores y bases de datos

- `net/http` lanza una goroutine por conexión **sin límite**: las goroutines son baratas, el recurso de detrás no. Propaga `r.Context()`, limita la concurrencia donde está el recurso, **descarta pronto** (503 + `Retry-After`), acota las colas, pon *timeouts* y apaga con `Shutdown`.
- `database/sql`: `*sql.DB` es un **pool** (ponle `SetMaxOpenConns`); cierra siempre las filas; **dentro de una transacción usa `tx`, no `db`** (agotamiento ⇒ interbloqueo); liga la transacción a un `ctx` (la cancelación hace `ROLLBACK`).

## Hallazgos del módulo (aprendidos midiendo)

- **Deadlock parcial silencioso:** 0 de 2 transferencias a los 300 ms y el runtime sin avisar; `pprof` señala las dos líneas exactas.
- **Livelock:** 216 480 intentos fallidos (6.7 s) sin backoff frente a 1 con backoff aleatorio. **Inversión de prioridad:** 156 ms → 23 ms con herencia (6.9×). **Inanición:** 364 ms → 52 ms con envejecimiento.
- **Filósofos:** el mismo código ingenuo se interbloquea el **100 %** o el **0 %** de las veces según una variable de temporización (`comer = 0` o `100 µs`).
- **Lectores–escritores:** con preferencia de lectores, el escritor hizo **2** operaciones en 600 ms; `sync.RWMutex` no sufre inanición y es el doble de rápido que el torniquete casero.
- **Barbero:** con un barbero sobrecargado, pasar de 2 a 10 sillas bajó las pérdidas del 38 % al 35 % pero multiplicó **×7** la espera; un segundo barbero las bajó al 5 %.
- **Rings:** el SPSC pasó de 50 ns a 12.8 ns (3.9×) con una copia local del contador del otro extremo, y supera 2.7× al channel; con 4+4 el MPMC (155 ns) **pierde** frente al channel (45 ns).
- **Mapa fragmentado:** 4.5× más rápido que el cerrojo global (95 % lecturas); con una clave caliente (30 % del tráfico) la mejora desaparece.
- **Skip list:** escala 3.8× con 8 goroutines; el slice con `RWMutex` **empeora** al añadir goroutines (134 → 200 ns); con 8 hilos la skip list es 4.1×-6.7× más rápida.
- **Lock-free:** el ABA corrompe la pila **sin pánico ni `race`**; el bucle CAS reintentó **hasta 5 082 veces seguidas**; ni la pila de Treiber ni la cola MS batieron al mutex, y el channel fue **5×** más rápido que todos.
- **APIs:** 100 constructores que lanzan goroutines ⇒ +100 fugas; el iterador no deja ninguna; `DescargarMal` vuelve a los 21 ms con **9 goroutines vivas**, `DescargarTodas` con 0.
- **HTTP bajo sobrecarga (capacidad 400 op/s):** sin control 49 ok/s y 1 364 operaciones inútiles; con límite + descarte 366 ok/s; con cola corta 391 ok/s (98 %).
- **`database/sql`:** el rendimiento fue 47 → 3 045 consultas/s de 1 a 64 conexiones; con `pool = goroutines` y una consulta fuera de la tx, **0 de 4** terminan; `MaxIdleConns = 0` ⇒ 100 aperturas frente a 1.

## Preguntas de repaso

1. ¿Cuáles son las cuatro condiciones de Coffman y cuál rompe el orden global de cerrojos?
2. ¿Por qué el runtime de Go no detecta un deadlock en un servidor real?
3. ¿Qué diferencia hay entre un livelock y una inanición? Pon un ejemplo de cada uno.
4. ¿Cómo aparece la inversión de prioridad en un lenguaje sin prioridades de hilo?
5. ¿Por qué `sync.RWMutex` bloquea los `RLock` nuevos cuando hay un `Lock` esperando?
6. En el barbero dormilón, ¿por qué añadir sillas no arregla un barbero sobrecargado?
7. ¿Por qué un ring SPSC guarda una copia local del contador del otro extremo?
8. ¿Por qué la fragmentación no ayuda con una clave caliente?
9. ¿Qué dos banderas hacen que `Contains` de la skip list perezosa no necesite cerrojo?
10. ¿Qué es el ABA y por qué no ocurre con punteros en Go?
11. ¿Qué significa «lock-free no es wait-free»? ¿Qué número del ejercicio 8 lo demuestra?
12. ¿Por qué es mejor que un constructor no lance goroutines?
13. ¿Qué garantiza `errgroup.Wait()` que `DescargarMal` no daba?
14. ¿Por qué descartar con 503 da más throughput útil que aceptar y encolar todo?
15. ¿Por qué usar `db` dentro de una transacción puede interbloquear el pool?

## Lecturas

- *The Little Book of Semaphores* — Allen B. Downey (filósofos, lectores–escritores, barbero, torniquetes).
- *The Art of Multiprocessor Programming* — Herlihy & Shavit (skip list perezosa, listas, colas, ABA, progreso).
- [Simple, Fast, and Practical Non-Blocking and Blocking Concurrent Queue Algorithms](https://www.cs.rochester.edu/u/scott/papers/1996_PODC_queues.pdf) — Michael & Scott.
- [Bounded MPMC queue](https://www.1024cores.net/home/lock-free-algorithms/queues/bounded-mpmc-queue) — Dmitry Vyukov.
- [Go Concurrency Patterns: Context](https://go.dev/blog/context) y [Pipelines and cancellation](https://go.dev/blog/pipelines).
- [Notes on structured concurrency](https://vorpus.org/blog/notes-on-structured-concurrency-or-go-statement-considered-harmful/) — Nathaniel J. Smith.
- [Managing connections](https://go.dev/doc/database/manage-connections) y [Canceling in-progress database operations](https://go.dev/doc/database/cancel-operations) (Go docs).
- *Release It!* — Michael Nygard (límites, *timeouts*, *bulkheads*, *load shedding*).
- Módulos anteriores: 5 (`context`), 6-7 (mutex y atómicos), 8 (patrones), 11 (`synctest`, `goleak`), 13 (contención y *false sharing*), 14 (scheduler y mutex por dentro).
