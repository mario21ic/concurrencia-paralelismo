# Ejercicio 2: llamadas bloqueantes y *netpoller*

## Teoría

Esperar no cuesta lo mismo según **cómo** se espera.

| Espera | Qué hace el runtime | Coste en hilos del SO |
|--|--|--|
| `<-ch`, `mu.Lock()`, `wg.Wait()`, `time.Sleep` | **aparca** la G (`gopark`); el M sigue ejecutando otra | **ninguno** |
| E/S de red o de `os.Pipe` **no bloqueante** | registra el descriptor en el ***netpoller*** (kqueue/epoll/IOCP); aparca la G; un hilo vigila todos | **ninguno** (un hilo del netpoller para miles de conexiones) |
| **Syscall bloqueante** (`read(2)` de un fichero, cgo, `Sleep` del kernel) | el M entra en el kernel **con su P**; tras ~20 µs `sysmon` ve que sigue bloqueado y le **quita el P** (*handoff*) para dárselo a otro M (si no hay, **crea un hilo**) | **uno por llamada bloqueada** |

### El *handoff* de P

Cuando una G entra en una syscall, el M conserva el P brevemente. Si tarda, `sysmon` hace *retake*: el P pasa a otro M (existente o nuevo) y sigue ejecutando goroutines. Al volver de la syscall, la G intenta recuperar un P; si no hay, se encola y el M se aparca. Esto evita que **una syscall lenta paralice todo el programa**, pero cada syscall bloqueada **retiene un hilo**.

### Límites

- `debug.SetMaxThreads` (por defecto **10 000**): si el programa lo supera, `fatal error: thread exhaustion` (no recuperable).
- Cada hilo ocupa ≈ 8 MB de espacio virtual reservado (pila del SO) y estructuras del kernel.
- Por eso en Go la E/S de red usa el netpoller, y para ficheros (que el SO no permite *poll*) **limita la concurrencia** (un pool/semáforo) en vez de lanzar miles de lecturas.

## Qué hace el programa

| Modo | Experimento |
|--|--|
| `-modo=comparar` | N goroutines bloqueadas de 6 formas distintas; cuenta los hilos del proceso con `runtime/metrics` (`/sched/threads/total:threads`) antes y después |
| `-modo=progreso` | con `GOMAXPROCS=1`, una goroutine bloqueada 300 ms en `read(2)`; otra cuenta pasos y se comprueba que **sigue avanzando** |
| `-modo=limite` | `SetMaxThreads(40)` y 100 goroutines en `read(2)`: provoca `thread exhaustion` |

```bash
go run . -modo=comparar
go run . -modo=progreso
go run . -modo=limite        # termina con un fatal error (esperado)
```

## Resultados de referencia (Apple M1, macOS, Go 1.26.5)

```
tipo de espera                     goroutines  hilos antes  hilos después
bloqueadas en un channel               100          5            7
bloqueadas en un channel              1000          5            9
en time.Sleep                          100          5            6
en time.Sleep                         1000          6            8
leyendo un os.Pipe (netpoller)         100          5            6
leyendo un os.Pipe (netpoller)        1000          5            6
en read(2) bloqueante (syscall)        100          5          103
en read(2) bloqueante (syscall)       1000          5         1005

GOMAXPROCS=1, 300 ms en read(2): la otra goroutine avanzó 2530 pasos (424 en 50 ms previos: mismo ritmo)

SetMaxThreads(40) y 100 goroutines en read(2)...  → fatal error: thread exhaustion
```

## Conclusiones

- **1 000 goroutines en channel, `Sleep` o netpoller usan 6-9 hilos; 1 000 en una syscall bloqueante usan 1 005**: un hilo por llamada. Es la diferencia entre "barato" y "caro".
- **El *handoff* funciona:** con un único P, la goroutine bloqueada en el kernel no detuvo a la otra (mismo ritmo de pasos). El P se cedió a otro hilo.
- **`SetMaxThreads` es un límite duro**: el programa muere, no recibe un error. En producción, un bug que bloquee miles de lecturas de fichero puede acabar así.
- Consecuencias de diseño: **limita la concurrencia de las operaciones que bloquean hilos** (ficheros, cgo, DNS con `cgo`) con un semáforo, y deja que la red use el netpoller.

## Ejercicios propuestos

1. Sustituye `read(2)` por lecturas de un fichero grande con `os.ReadFile` y mide hilos con 1 000 goroutines; limita con un semáforo de 32 y compara.
2. Fuerza `GODEBUG=netdns=cgo` y cuenta hilos al resolver 500 nombres a la vez.
3. Mide el coste (µs) de un *handoff* cronometrando la reanudación de una G tras una syscall corta frente a una larga.
4. Observa con `GODEBUG=schedtrace=100` cómo crece `threads=` durante `-modo=comparar`.
