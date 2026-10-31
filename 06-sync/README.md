# Módulo 6. Paquete `sync`

**Objetivo:** proteger estado compartido con las primitivas de `sync`, saber cuál elegir y evitar sus errores clásicos.

## Ejercicios

| # | Carpeta | Tema |
|---|---------|------|
| 1 | [01-mutex](01-mutex) | `Mutex`, invariantes, orden de locks, `TryLock` |
| 2 | [02-rwmutex](02-rwmutex) | `RWMutex`: cuándo ayuda (medido) y el deadlock del `RLock` recursivo |
| 3 | [03-once](03-once) | `Once`, `OnceFunc`, `OnceValue`, `OnceValues` |
| 4 | [04-cond](04-cond) | `Cond`: cola acotada y `Broadcast` |
| 5 | [05-sync-map](05-sync-map) | `sync.Map`: API y benchmarks |
| 6 | [06-pool](06-pool) | `sync.Pool`: reutilizar objetos, benchmarks |
| 7 | [07-check-then-act](07-check-then-act) | Componer operaciones seguras **no** es seguro |

```bash
cd 06-sync/01-mutex && go run .
cd 06-sync/05-sync-map && go test -bench . -benchmem   # ejercicios 5 y 6
```

> Mediciones: Mac M1 de 8 núcleos, Go 1.26.5. Los números absolutos cambian de una máquina a otra; compara las **proporciones** y mide siempre en tu hardware.

## Resumen teórico

| Primitiva | Para qué | Notas clave |
|--|--|--|
| `Mutex` | exclusión mutua | no reentrante; no copiar; `defer Unlock()` |
| `RWMutex` | muchos lectores **o** un escritor | no se puede "subir" de RLock a Lock; `RLock` recursivo puede dar deadlock |
| `Once` / `OnceFunc` / `OnceValue(s)` | inicialización perezosa única | el error de `OnceValues` también se cachea |
| `Cond` | esperar una condición compleja sobre estado protegido | casi siempre hay alternativa con channels |
| `Map` | mapa concurrente para casos específicos | valores `any`; no es un reemplazo general de `map`+`Mutex` |
| `Pool` | caché de objetos temporales para aliviar el GC | el GC puede vaciarlo; hay que resetear el objeto |
| `WaitGroup` | esperar a un grupo | ya visto en el módulo 2 |

### Principios

1. **Un mutex protege invariantes, no variables sueltas.** Define qué datos protege y qué propiedad debe cumplirse cuando se libera el lock. Pon el mutex junto a los campos que protege, y no los expongas.
2. **Secciones críticas cortas.** No hagas I/O, llamadas de red ni esperes channels con un lock tomado.
3. **`defer mu.Unlock()`** justo después de `Lock()`: sobrevive a `return` y a `panic`.
4. **No copiar** un `Mutex`/`WaitGroup`/`Cond`/`Pool`/`Map` tras el primer uso (`go vet` → `copylocks`). Usa punteros o estructuras no copiables.
5. **Orden global de locks** si necesitas varios (A antes que B siempre) para evitar deadlock.
6. **Componer no es atómico:** dos operaciones individualmente seguras no forman una operación segura (check-then-act). Ofrece operaciones compuestas dentro de una sola sección crítica.
7. **No llames a código desconocido** (callbacks, interfaces) con un lock tomado: puede volver a entrar y bloquearse.
8. **Mide antes de elegir** `RWMutex`, `sync.Map` o `Pool`: su ventaja depende de la carga.

### ¿Mutex, channel o atomic?

| Situación | Elección |
|--|--|
| Proteger un estado compartido con invariantes | `Mutex` |
| Transferir propiedad de datos, pipelines, eventos | channel |
| Un contador o flag simple | `atomic` (módulo 7) |
| Muchas lecturas, pocas escrituras, trabajo de lectura apreciable | `RWMutex` (medir) |
| Claves estables y lecturas dominantes, o conjuntos de claves disjuntos por goroutine | `sync.Map` |
| Objetos temporales caros de asignar | `sync.Pool` |

## Preguntas de repaso

1. ¿Por qué `sync.Mutex` no es reentrante y qué ocurre si haces `Lock` dos veces seguidas?
2. ¿Qué es un invariante y cómo lo protege un mutex?
3. Dos goroutines transfieren A→B y B→A. ¿Cómo se produce el deadlock y cómo se evita?
4. ¿Por qué un segundo `RLock` en la misma goroutine puede bloquearse?
5. ¿Por qué `Cond.Wait` se usa siempre dentro de un `for`?
6. ¿Qué pasa si la función de `Once.Do` entra en pánico? ¿Y con `OnceFunc`?
7. ¿Cuándo es `sync.Map` mejor que `map` + `RWMutex`?
8. ¿Por qué no puedes confiar en que un objeto devuelto a un `Pool` siga ahí?

## Lecturas

- Documentación de [`sync`](https://pkg.go.dev/sync)
- [The Go Memory Model](https://go.dev/ref/mem) (garantías de `Mutex`, `Once`, `WaitGroup`)
- *Go Data Race Detector*: <https://go.dev/doc/articles/race_detector>
- Artículo del blog: *Go 1.24: new sync.Map implementation* (en las notas de la versión)
