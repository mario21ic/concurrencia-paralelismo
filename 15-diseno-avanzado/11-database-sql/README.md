# Ejercicio 11: concurrencia y bases de datos (`database/sql`)

> Para que el ejercicio funcione **sin instalar nada**, usa un **driver simulado** (`fakedb.go`) que implementa `database/sql/driver`: cada consulta tarda un tiempo configurable, respeta el contexto y cuenta aperturas, cierres, commits y rollbacks. El comportamiento del **pool**, de las **transacciones** y de la **cancelación** es el de la biblioteca estándar real; solo la base de datos es de mentira.

## Teoría

### `*sql.DB` es un pool, no una conexión

- Es **seguro para uso concurrente** y se crea **una vez** por base de datos (no por solicitud); `db.Close()` al terminar el programa.
- Gestiona un conjunto de conexiones: abre bajo demanda, reutiliza las ociosas, cierra las caducadas.
- Cada llamada (`QueryContext`, `ExecContext`…) **toma una conexión del pool y la devuelve al terminar**. Si no hay ninguna libre y se alcanzó el máximo, **la goroutine espera** (sin límite, salvo `ctx`).

| Ajuste | Por defecto | Qué hace |
|--|--|--|
| `SetMaxOpenConns(n)` | **ilimitado** | máximo de conexiones simultáneas; **ponlo siempre** (la BD tiene su propio máximo) |
| `SetMaxIdleConns(n)` | 2 | conexiones que se conservan abiertas e inactivas; si es menor que la concurrencia habitual, se abren y cierran sin parar |
| `SetConnMaxLifetime(d)` | sin límite | recicla conexiones viejas (balanceadores, *failover*, cambios de DNS) |
| `SetConnMaxIdleTime(d)` | sin límite | cierra las ociosas tras `d` |

`db.Stats()` expone `OpenConnections`, `InUse`, `Idle`, `WaitCount`, `WaitDuration`, `MaxIdleClosed`…: **observa `WaitCount/WaitDuration`**; si crecen, el pool es tu cuello de botella.

### Tamaño del pool

La concurrencia máxima útil es **`MaxOpenConns / latencia`** (ley de Little). Más conexiones **no** son gratis: cada una consume memoria y un proceso/hilo en la BD, y una BD saturada rinde **peor**, no mejor. Regla práctica: empieza con un valor modesto (≈ 2-4× núcleos de la BD o el que ésta recomiende), **limita la concurrencia de entrada** (ejercicio 10) para que el pool no sea la cola, y ajusta midiendo `WaitDuration`.

### Cosas que se retienen una conexión

| Objeto | La conexión se devuelve cuando… |
|--|--|
| `db.QueryContext` → `*sql.Rows` | `rows.Close()` (o al agotar las filas con `Next() == false`, o por error) |
| `db.BeginTx` → `*sql.Tx` | `Commit()` o `Rollback()` |
| `db.Conn(ctx)` → `*sql.Conn` | `conn.Close()` |

**Olvidar `defer rows.Close()`, o salir de un `for rows.Next()` antes de tiempo sin cerrar, es la fuga de conexiones más común.** Y después de un bucle, comprueba `rows.Err()`.

### Interbloqueo por agotamiento del pool

El error de diseño más traicionero: **retener una conexión mientras se pide otra**.

```go
tx, _ := db.BeginTx(ctx, nil)       // retiene la conexión 1
rows, _ := db.QueryContext(ctx, …)  // ← usa db, NO tx: pide una conexión 2
```

Con `MaxOpenConns = N` y `N` goroutines haciéndolo a la vez, todas retienen una y esperan otra: **nadie avanza** y ni el runtime ni la BD lo detectan. Es el *hold-and-wait* de Coffman (ejercicio 1). Soluciones:

1. **Dentro de una transacción, usa siempre `tx`**, nunca `db`.
2. Limita la concurrencia de entrada a `MaxOpenConns − 1` (o un margen holgado).
3. Pasa siempre un `ctx` con plazo: convierte el interbloqueo en un error.
4. Evita funciones que abran su propia consulta en medio de una transacción (inyecta una interfaz `Querier` implementada por `*sql.DB` y `*sql.Tx`).

### Transacciones y cancelación

- Un `*sql.Tx` está ligado a **una** conexión; no lo uses desde varias goroutines a la vez (se serializan, y casi seguro es un error de diseño).
- `db.BeginTx(ctx, …)`: si `ctx` se cancela, **`database/sql` hace ROLLBACK automáticamente** y `Commit` devuelve `sql.ErrTxDone`. Úsalo para ligar el ciclo de vida de la transacción a la solicitud.
- Las consultas con `QueryContext`/`ExecContext` se **cancelan en el servidor** si el driver lo soporta; la conexión se devuelve al pool.
- Patrón: `defer tx.Rollback()` justo tras `BeginTx` (es inocuo tras un `Commit`) para no dejar transacciones abiertas en un `return` temprano.
- Reintenta transacciones que fallen por **serialización/deadlock de la BD** (códigos específicos del motor) con *backoff*, no en bucle cerrado.

## Qué hace el programa

| Experimento | Qué muestra |
|--|--|
| A) `ExpPool` | consultas/s según `MaxOpenConns` |
| B) `ExpAgotamiento` | el interbloqueo por retener una conexión y pedir otra; tres remedios |
| C) `ExpFilasSinCerrar` | con un pool de 1, una fila sin cerrar bloquea la siguiente consulta |
| D) `ExpCancelarTx` | cancelar el `ctx` hace `ROLLBACK` y `Commit` falla con `ErrTxDone` |
| E) `ExpOciosas` | `MaxIdleConns` bajo ⇒ una conexión nueva por consulta |

```bash
go run .
go test -race -v .
```

## Resultados de referencia

```
== A) 64 goroutines × 10 consultas de 20 ms ==
  MaxOpenConns  consultas/s  veces que esperó   espera media
  1                   47            638             1.086s
  2                   95            637              543ms
  4                  189            633              264ms
  8                  365            623              129ms
  16                 692            612               56ms
  32                1317            570               20ms
  64                3045              0                0s

== B) 4 goroutines con una transacción abierta + una consulta dentro, plazo 400 ms ==
  consulta con db, pool = 4  → terminadas 0, vencieron el plazo 4     (interbloqueo)
  consulta con db, pool = 5  → terminadas 4, vencieron 0
  consulta con db, pool = 8  → terminadas 4, vencieron 0
  consulta con tx, pool = 4  → terminadas 4, vencieron 0

== C) filas sin cerrar, pool de 1:   sin Close → 1 completada, context deadline exceeded | con Close → 5 completadas
== D) cancelar el ctx de una tx:     ROLLBACK 1, COMMIT 0, Commit() → sql.ErrTxDone
== E) 100 consultas seguidas, MaxOpenConns=8:  MaxIdleConns=0 → 100 aperturas | 1, 2 u 8 → 1 apertura
```

## Conclusiones

- **El rendimiento es lineal en el tamaño del pool hasta que deja de ser el cuello de botella**: 47 → 3 045 consultas/s de 1 a 64 conexiones (≈ `n / 20 ms`). La espera media en el pool (1 086 ms con 1 conexión) es **tiempo de cola**, no de consulta: aparece en `Stats().WaitDuration`.
- **El interbloqueo por agotamiento es total**: con `pool = goroutines` y una consulta fuera de la transacción **0 de 4** terminan (solo el plazo del `ctx` los libera). **Un solo hueco de margen (pool = 5)** lo evita en este caso (no en general: con 8 goroutines haría falta 9), y la solución robusta es usar `tx` dentro de la transacción.
- **Olvidar `rows.Close()` con un pool de 1 detiene el sistema tras la 1.ª consulta**; con más conexiones, simplemente se van perdiendo hasta llegar al mismo punto. Se detecta con `Stats().InUse` que no baja.
- **Cancelar el contexto es la forma limpia de abortar una transacción**: el `ROLLBACK` llega al driver sin que el código lo pida, y `Commit` devuelve `sql.ErrTxDone`.
- **`MaxIdleConns = 0` (o menor que la concurrencia real) multiplica por 100 las aperturas**: aquí 100 conexiones frente a 1. En una BD real cada apertura son TCP + TLS + autenticación (milisegundos), un coste fijo que se paga por consulta.

## Ejercicios propuestos

1. Añade un semáforo `chan struct{}` de tamaño `MaxOpenConns − 1` delante de las transacciones y repite B con `pool = goroutines`.
2. Implementa una interfaz `Querier` (`QueryContext`, `ExecContext`) satisfecha por `*sql.DB` y `*sql.Tx` y refactoriza `consultar` para que reciba la correcta.
3. Escribe un `ReintentarTx(ctx, db, f)` que repita la transacción ante un error "reintentable" con *backoff* exponencial y *jitter*, respetando `ctx`.
4. Con un Postgres real (`pgx/stdlib`), mide `Stats().WaitDuration` bajo carga con distintos `SetMaxOpenConns` y compara con las cifras simuladas.
