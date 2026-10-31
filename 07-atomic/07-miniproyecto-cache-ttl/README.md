# Mini-proyecto 2: caché en memoria con TTL y expiración por `context`

## Enunciado

Una caché genérica `Cache[K, V]` segura para uso concurrente que:

- guarda valores con **TTL por entrada** (`ttl <= 0` = no expira),
- expira **perezosamente** en `Get` y también con un **barredor en segundo plano**,
- el barredor termina al **cancelar un `context`** (sin fugas de goroutines),
- expone **estadísticas** (aciertos, fallos, expiradas) sin tomar el lock,
- se puede probar **sin dormir**, inyectando un reloj.

```bash
go run .
go test -race -count=3 .
go test -bench . -run xxx
```

## API

```go
c := Nueva[string, int](ctx, 20*time.Millisecond) // intervalo del barredor (0 = sin barredor)
c.Set("k", 1, 5*time.Second)                      // 0 = no expira
v, ok := c.Get("k")
c.Delete("k")
c.Len()                                           // incluye vencidas aún no barridas
c.Barrer()                                        // quita vencidas, devuelve cuántas
c.Stats()                                         // {Aciertos, Fallos, Expiradas}
<-c.Hecho()                                       // se cierra al terminar el barredor
```

## Diseño

```
        Set / Get / Delete / Barrer / Len
                 │ (toman mu)
                 ▼
          ┌─────────────┐        ┌────────────────────┐
          │ map[K]entry │◄───────│ barredor (goroutine)│◄── ticker
          └─────────────┘        └────────┬───────────┘
        protegido por sync.Mutex          │ termina con ctx.Done()
                                          ▼
    aciertos / fallos / expiradas   close(hecho)
        (atomic.Int64, sin lock)
```

Decisiones y por qué:

1. **Un mutex para el map**, no `sync.Map` ni atomics. `Get` puede *borrar* (si la entrada venció), es decir, leer-comprobar-escribir: hay una invariante (map y expiración) y por tanto un mutex es lo correcto (módulos 6 y 7).
2. **Contadores atómicos** para las estadísticas: son valores independientes que no necesitan estar bajo el lock y se leen sin bloquear a nadie. Los incrementos de `aciertos`/`fallos` se hacen **fuera** de la sección crítica para acortarla.
3. **Doble expiración.** La perezosa en `Get` garantiza **corrección** (nunca devuelve un valor vencido, aunque el barredor vaya atrasado); el barredor garantiza **memoria acotada** (las claves que nadie vuelve a pedir también se eliminan).
4. **`context` para el ciclo de vida.** El barredor es el único lugar donde se lanza una goroutine y siempre tiene salida (`ctx.Done()`), `defer t.Stop()` y señal `hecho`.
5. **Reloj inyectable** (`ahora func() time.Time`). Los tests avanzan el tiempo con un reloj falso: rápidos, deterministas, sin `time.Sleep`. Solo el test del barredor real espera (con límite).
6. **Expiración exacta en el límite:** una entrada con TTL 10 s deja de ser válida **en** el segundo 10 (`!ahora.Before(exp)`).
7. **Genéricos** para evitar `any` y aserciones.

## Tests (`cache_test.go`)

| Test | Qué verifica |
|--|--|
| `TestSetGet` | operaciones básicas |
| `TestExpiracionPerezosa` | vive a 9 s, muere a 10 s; `Get` elimina; contador |
| `TestSinTTLNoExpira` | `ttl=0` no expira |
| `TestBarrer` | elimina solo las vencidas |
| `TestSobrescribirRenuevaTTL` | `Set` renueva el plazo |
| `TestDelete` | semántica de retorno |
| `TestStats` | contadores |
| `TestBarredorYCancelacion` | el barredor elimina y termina al cancelar |
| `TestSinFugaDeGoroutines` | 50 cachés creadas y canceladas no dejan goroutines |
| `TestConcurrencia` | 16 goroutines mezclando `Set/Get/Delete/Barrer` (con `-race`) |

## Resultado de referencia

```
Get corta (recién creada): 1 true
Len tras 120ms (sin llamar a Get): 2 (el barredor ya quitó 'corta')
Get corta tras 120ms:              false
stats: aciertos=59851 fallos=151 expiradas=1 (len=102)
barredor detenido tras cancelar el contexto

BenchmarkGetParalelo-8   195 ns/op      # 8 goroutines leyendo la misma caché
```

(Los números de `stats` varían ligeramente entre ejecuciones.)

## Limitaciones conocidas (extensiones propuestas)

1. **Contención del mutex:** `BenchmarkGetParalelo` da ~195 ns/op porque todas las lecturas pasan por un único lock. **Fragmenta** (sharding) la caché en N mapas con su propio mutex y elige el fragmento con `hash/maphash` (módulo 13).
2. **`Barrer` recorre todo el map con el lock tomado:** con millones de entradas bloquea a los demás. Alternativas: barrer por lotes, un heap de vencimientos, o fragmentos.
3. **Sin límite de tamaño ni política de expulsión** (LRU/LFU). Añade capacidad máxima.
4. **Sin `GetOrCompute`:** si 100 goroutines piden la misma clave ausente, las 100 calcularán el valor. Solución: `singleflight` (módulo 10).
5. **Callback de expulsión** (`onEvict`) para liberar recursos al vencer una entrada.
6. **Expiración deslizante** (renovar TTL en cada acierto).
7. Sustituir el ticker por `time.AfterFunc` por entrada, y valorar el coste de millones de temporizadores.
