# Ejercicio 7: `sync.Mutex` por dentro — modo normal y modo de inanición

## Teoría

`sync.Mutex` ocupa **8 bytes**: un `state int32` y un semáforo `sema uint32`. El estado empaqueta:

```
state:  [ nº de goroutines esperando … | starving | woken | locked ]
bit 0 (locked)   el cerrojo está tomado
bit 1 (woken)    hay una goroutine despertada que ya está intentando tomarlo
bit 2 (starving) modo de inanición
resto            cuántas esperan
```

### Camino rápido

`Lock()` hace un `CompareAndSwap(0 → locked)`: si funciona (sin contención), son unos pocos ns y no entra el planificador. `Unlock()` hace un `Add(-locked)`; si nadie espera, termina.

### Contención: dos modos

| Modo | Reglas | Pros / contras |
|--|--|--|
| **Normal** (por defecto) | las goroutines que llegan **compiten** con la despertada (*barging*); antes de dormirse, una goroutine **gira** unas iteraciones (*spinning*) si hay otros núcleos | máximo rendimiento (la que ya corre en CPU suele ganar y evita un cambio de contexto); **injusto**: una goroutine recién despertada puede perder una y otra vez |
| **Inanición** | si un waiter lleva **> 1 ms** esperando, el mutex pasa a este modo: `Unlock` **entrega** el cerrojo directamente al primer waiter (FIFO); los que llegan **no giran ni compiten**, se ponen al final | **justo**; cada traspaso exige despertar a una goroutine (≈ µs): menos operaciones por segundo |

Vuelve a modo normal cuando el waiter que lo recibe es el **último** de la cola, o cuando esperó **< 1 ms**.

### Por qué importa

Sin el modo de inanición, un mutex muy disputado con secciones críticas largas podría dejar a una goroutine esperando mucho tiempo. Con él hay una **cota práctica** a la espera: ≈ 1 ms + las secciones críticas ya en cola. Un cerrojo CAS puro (*spinlock*) no tiene cola ni garantías.

### Recomendaciones

- Secciones críticas **cortas**; no hagas E/S ni llames a código desconocido con el cerrojo tomado.
- Un `Mutex` no es reentrante ni se copia (`go vet` avisa). `TryLock` existe (Go 1.18) pero casi siempre indica un problema de diseño.
- Si la contención es el problema, no optimices el mutex: **reduce el estado compartido** (módulo 13).

## Qué hace el programa

`escenario` lanza `GOMAXPROCS − 2` goroutines "acaparadoras" que toman el cerrojo en bucle sin pausa, y una "víctima" que lo pide cada 5 ms y mide cuánto espera. Se compara `sync.Mutex` con un `spin` (cerrojo CAS sin cola) con dos tamaños de sección crítica.

```bash
go run .
```

## Resultados de referencia (Apple M1, 6 acaparadoras, 2 s por configuración, Go 1.26.5)

```
sizeof(sync.Mutex) = 8 bytes

sección crítica de 200 ns:
  sync.Mutex    víctima: p50=1.08µs   p99=105µs    máx=5.97ms  | 5 822 270 ops (reparto mín/máx = 0.98)
  spinlock CAS  víctima: p50=499µs    p99=7.18ms   máx=25.7ms  | 4 420 758 ops (reparto = 0.81)

sección crítica de 50 µs:
  sync.Mutex    víctima: p50=646µs    p99=1.11ms   máx=1.91ms  | 39 002 ops (reparto = 0.60)
  spinlock CAS  víctima: p50=43.6ms   p99=194ms    máx=438ms   | 35 807 ops (reparto = 0.29)
```

## Conclusiones

- **El modo de inanición pone una cota a la espera:** con secciones de 50 µs, la víctima espera **p99 = 1.1 ms (máx 1.9 ms)** con `sync.Mutex` frente a **p99 = 194 ms (máx 438 ms)** con el spinlock.
- **Con secciones muy cortas (200 ns) casi no se nota la injusticia**: el mutex ofrece p50 = 1 µs porque la víctima entra en un hueco; el spinlock tiene p50 = 499 µs.
- **El mutex gana también en rendimiento total** (5.8 M frente a 4.4 M ops con secciones de 200 ns): el spinlock desperdicia CPU y empeora la contención en la línea de caché.
- **Reparto entre acaparadoras:** 0.98 con el mutex frente a 0.81 con el spinlock (1.0 = perfectamente equitativo).
- Un cerrojo "sin cola" es una apuesta: puede ir bien o dejar a una goroutine sin servicio durante cientos de milisegundos.

## Ejercicios propuestos

1. Reproduce el cambio de modo: registra con `runtime/metrics` (`/sync/mutex/wait/total:seconds`) cuánto esperan las goroutines y relaciónalo con la sección crítica.
2. Añade un cerrojo **ticket** (FIFO estricto con contador atómico) y compara justicia y rendimiento.
3. Activa el perfil de mutex (`runtime.SetMutexProfileFraction`) y localiza la línea disputada con `go tool pprof`.
4. Mide con una sola acaparadora: ¿aparece la inanición?
