# Módulo 14. Internals del runtime

**Objetivo:** entender **por qué** el runtime de Go se comporta como se comporta —planificador G-M-P, llamadas bloqueantes, preemption, stacks, channels, `select` y mutex— observándolo con experimentos reproducibles, no leyendo solo teoría.

## Ejercicios

| # | Carpeta | Tema |
|---|---------|------|
| 1 | [01-gmp-colas](01-gmp-colas) | G-M-P: `runnext`, cola local, cola global, *work stealing* |
| 2 | [02-syscalls-y-netpoller](02-syscalls-y-netpoller) | Syscall bloqueante frente a netpoller, *handoff* de P, límite de hilos |
| 3 | [03-preemption](03-preemption) | Preemption cooperativa frente a asíncrona; latencia de despertar |
| 4 | [04-lockosthread](04-lockosthread) | `LockOSThread`: hilo principal, coste y fugas de hilos |
| 5 | [05-stacks](05-stacks) | Crecimiento por copia, reducción en el GC, ~2 KB por goroutine |
| 6 | [06-canales-y-select](06-canales-y-select) | `hchan`, colas de espera FIFO, `selectgo`, coste frente a mutex |
| 7 | [07-mutex](07-mutex) | `sync.Mutex`: modo normal frente a modo de inanición |
| 8 | [08-del-codigo-al-runtime](08-del-codigo-al-runtime) | Qué función de `runtime.*` llama el compilador por cada construcción |

```bash
cd 14-internals/01-gmp-colas && go run . -modo=orden
```

> **Hardware y versión de las mediciones:** Apple M1, 8 núcleos, macOS (Darwin), Go 1.26.5. Las **cifras** cambian con tu máquina y con la versión del runtime; lo reutilizable es el **método** y la **forma** de los resultados. Los internals **no son API**: pueden cambiar entre versiones.

## Resumen teórico

### 1. El modelo G-M-P

| Pieza | Qué es | Detalle clave |
|--|--|--|
| **G** | goroutine | stack propio (empieza en ~2 KB) y estado; vive en colas |
| **M** | hilo del SO | ejecuta código; si hace una syscall bloqueante, se queda con ella |
| **P** | procesador lógico (`GOMAXPROCS`) | **cola local** (256 huecos) + ranura `runnext`; sin P no se ejecuta Go |

- Una G recién creada va a **`runnext`** del P actual; la que ocupaba esa ranura pasa al final de la cola local. Por eso, con un solo P, `A..H` se ejecutan como **`H A B C D E F G`**.
- Si la cola local se llena, mitad de ella se vuelca a la **cola global**. Un P sin trabajo busca: cola local → global → netpoller → **robar** a otro P.
- Cada 61 ticks el P mira la cola global para que no pase hambre.

### 2. Esperar no cuesta lo mismo

| Espera | Hilos del SO consumidos |
|--|--|
| Channel, mutex, `time.Sleep` | **ninguno** (la G se aparca; el hilo sigue con otra) |
| E/S de red o `os.Pipe` (**netpoller**: kqueue/epoll) | **ninguno** |
| **Syscall bloqueante** (`read(2)` en un fichero, cgo) | **uno por llamada**; el P se *entrega* (*handoff*) a otro M |

Con `GOMAXPROCS=1` otra goroutine sigue avanzando mientras una está en una syscall: es el *handoff*. El precio es el **hilo**; el límite por defecto es 10 000 (`debug.SetMaxThreads`) y se rebasa con error fatal.

### 3. Preemption

- **Cooperativa:** la G cede en puntos seguros (prólogo de funciones que comprueban el stack, channels, locks…).
- **Asíncrona** (desde Go 1.14): `sysmon` envía una señal al hilo si una G lleva >10 ms; permite interrumpir **bucles sin llamadas** que antes bloqueaban el programa.
- Con `GODEBUG=asyncpreemptoff=1`, un bucle así **no cede nunca**; con funciones de marco grande sí (el prólogo comprueba el stack).
- La latencia típica de despertar tras un acaparador es de unos **~10-25 ms**, no microsegundos.

### 4. `LockOSThread`

Fija una goroutine a **un** hilo y ese hilo a esa goroutine. Necesario cuando el estado vive **en el hilo**: GUI, OpenGL, `setns`, ciertas bibliotecas C con TLS. Reglas:

1. Hazlo en `init()` o al principio de `main()` para fijar el **hilo principal** (`main` empieza en él).
2. **Suéltalo** (`UnlockOSThread`): una G que termina sin soltar **destruye** el hilo; en un bucle, eso es una fábrica de hilos.
3. Es **caro** si la G se comunica mucho: cada traspaso exige un cambio de contexto del SO.

### 5. Stacks

- Empiezan en 2 KB y **crecen copiando**: se reserva uno del doble y se **reajustan todos los punteros** al stack. Por eso las direcciones de variables locales *cambian* y no puedes guardar un `uintptr` al stack.
- El GC **reduce** stacks que usan menos de ¼ de lo reservado.
- Límite por defecto 1 GB (`debug.SetMaxStack`); pasarlo es un **error fatal**, no recuperable con `recover`.
- Coste de memoria de una goroutine inactiva: **≈ 2 KB de stack** (+ el `g` y su contexto) ⇒ millones son posibles; cientos de millones, no.

### 6. Channels y `select`

- **`hchan`**: cabecera (~96 B) con contador, índices del **buffer circular**, un cerrojo y dos colas FIFO de **`sudog`** (`recvq`, `sendq`). Si el elemento no tiene punteros, cabecera y buffer van en **un solo bloque**.
- **Envío:** ¿hay un receptor esperando? ⇒ entrega **directa** (sin tocar el buffer). ¿Hay hueco? ⇒ al buffer. Si no ⇒ la G se aparca en `sendq`. Recibir es simétrico.
- **`close`** despierta a **todos** los bloqueados.
- **`select`** con ≥2 casos: `selectgo` ordena los channels por dirección, los **bloquea todos**, elige al azar entre los listos (**sin prioridad**) y los desbloquea. Con `default` y un caso, es una operación directa (`selectnbrecv`). `nil` no está nunca listo; cerrado siempre lo está.

### 7. `sync.Mutex`

- **8 bytes**: `state int32` (bits *locked*, *woken*, *starving* + nº de esperando) y un semáforo.
- **Modo normal:** un goroutine que llega puede **colarse** (*barging*) delante del despertado: máximo rendimiento, sin justicia.
- **Modo inanición:** si un esperando lleva **más de 1 ms**, el cerrojo se entrega en orden FIFO y nadie se cuela. El modo vuelve a normal cuando la cola se vacía o el último esperando no ha esperado >1 ms.
- Resultado: la espera tiene una **cota práctica**; un cerrojo CAS sin cola no ofrece ninguna.

### 8. Del código al runtime

| Código | Llamada |
|--|--|
| `go f()` | `runtime.newproc` |
| `make(chan T, n)` | `runtime.makechan` |
| `c <- v` / `<-c` | `runtime.chansend1` / `runtime.chanrecv1` (`chanrecv2` en `range` y `v, ok :=`) |
| `close(c)` | `runtime.closechan` |
| `select` (≥2 casos) | `runtime.selectgo` |
| `select` + `default` (1 caso) | `runtime.selectnbrecv` / `selectnbsend` |
| `mu.Lock()` | CAS en línea; `…lockSlow` si falla |
| `x.(I)` | `runtime.typeAssert` |

### Herramientas para mirar dentro

| Herramienta | Para qué |
|--|--|
| `GODEBUG=schedtrace=1000,scheddetail=1` | estado de Ps, Ms y colas cada N ms |
| `GODEBUG=asyncpreemptoff=1` | desactiva la preemption asíncrona (diagnóstico) |
| `runtime/trace` + `go tool trace` | línea temporal por P y por G |
| `runtime.NumGoroutine`, `runtime/metrics` | contadores del runtime |
| `go build -gcflags=-S` | ensamblador: qué llamadas a `runtime.*` genera tu código |
| `pprof` perfiles `goroutine` / `threadcreate` | quién está bloqueado y cuántos hilos se crearon |

## Hallazgos del módulo (aprendidos midiendo)

- **Orden con un P:** `H A B C D E F G` en 200 de 200 ejecuciones: `runnext` + FIFO es determinista.
- **Hilos del SO:** 1 000 goroutines en un channel, `Sleep` o `os.Pipe` ⇒ **5-9 hilos**; 1 000 en un `read(2)` bloqueante ⇒ **1 005 hilos**.
- **Preemption:** sin la asíncrona (`asyncpreemptoff=1`), un bucle sin llamadas **bloquea el programa para siempre**; con ella, `main` despierta a los ~23-25 ms.
- **`LockOSThread`:** ping-pong **96× más lento** (280 ns → 27 µs); 200 goroutines que terminan sin soltar el hilo ⇒ **200 hilos**; soltando, **1**.
- **Stacks:** 2 049 bytes por goroutine inactiva; la primera recursión profunda es **~8.7× más lenta** que la segunda (copias sucesivas); el GC redujo 336 MB → 84 MB.
- **Channels:** cabecera de ~111 B; con buffer, el tamaño añade `n × elemento` en el mismo bloque. FIFO de espera comprobado (`0..7` → `0..7`). `select` con 3 casos listos elige a **33.3 % / 33.4 % / 33.3 %**.
- **Coste de `select`:** ≈ 31 ns (1 caso), 61 ns (2), 105 ns (4), 277 ns (8): crece con el nº de casos.
- **Channel frente a mutex** (mismo nº de operaciones): con buffer 73 ns, sin buffer 133 ns, mutex 43 ns; el ping-pong sin buffer cuesta ~281 ns de ida y vuelta.
- **Mutex:** con secciones de 50 µs y 6 acaparadores, la víctima espera **p99 = 1.1 ms (máx 1.9 ms)** con `sync.Mutex` frente a **p99 = 194 ms (máx 438 ms)** con un spinlock CAS: el modo inanición *acota* la espera.

## Preguntas de repaso

1. ¿Por qué con un solo P la última goroutine creada se ejecuta la **primera**?
2. ¿Qué ocurre cuando la cola local de un P se llena? ¿Y cuando se queda vacío?
3. ¿Por qué mil goroutines en `time.Sleep` caben en 9 hilos y mil en un `read(2)` no?
4. ¿Qué es el *handoff* de P y cuándo evita que el programa se pare?
5. ¿Por qué un bucle `for {}` bloqueaba antes de Go 1.14 y ya no? ¿Qué lo reactiva con `asyncpreemptoff=1`?
6. ¿Por qué `LockOSThread` hace tan lento un ping-pong? ¿Cuándo compensa?
7. ¿Por qué cambian las direcciones de las variables locales? ¿Qué se rompería si no se ajustaran los punteros?
8. ¿Qué hace `selectgo` y por qué ordena los channels por dirección?
9. ¿Por qué un `select` con 8 casos cuesta ~9× lo que uno con 1?
10. ¿Cuándo pasa un `sync.Mutex` a modo de inanición, y qué se sacrifica?

## Lecturas

- Código fuente del runtime (`$GOROOT/src/runtime`): `proc.go`, `chan.go`, `select.go`, `stack.go`, `preempt.go`; y `internal/sync/mutex.go`.
- [Scalable Go Scheduler Design Doc](https://golang.org/s/go11sched) — Dmitry Vyukov
- [Go 1.14: *Non-cooperative goroutine preemption*](https://go.googlesource.com/proposal/+/master/design/24543-non-cooperative-preemption.md)
- [Contiguous stacks design doc](https://golang.org/s/contigstacks)
- [`runtime` package](https://pkg.go.dev/runtime) y [`runtime/debug`](https://pkg.go.dev/runtime/debug)
- Módulos anteriores: 2 (goroutines), 3 (channels), 6 (mutex), 12 (trazas y perfiles), 13 (rendimiento).
