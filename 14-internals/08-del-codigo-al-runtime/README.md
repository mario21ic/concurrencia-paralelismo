# Ejercicio 8: del código al runtime

## Teoría

Casi toda la «magia» de la concurrencia en Go es el **compilador convirtiendo sintaxis en llamadas a funciones de `runtime`**. Verlo desmitifica el lenguaje y ayuda a razonar sobre el coste.

| Código Go | Llamada que genera el compilador | Qué hace |
|--|--|--|
| `go f()` | `runtime.newproc` | crea (o reutiliza de `gFree`) una `g`, prepara su stack y la deja en `runnext` del P actual |
| `make(chan T, n)` | `runtime.makechan` | reserva `hchan` (+ buffer) |
| `c <- v` | `runtime.chansend1` | entrega a un receptor, o al buffer, o aparca (`gopark`) |
| `<-c` | `runtime.chanrecv1` (`chanrecv2` en `v, ok :=` y en `range`) | simétrico |
| `close(c)` | `runtime.closechan` | despierta a todos los bloqueados |
| `select` (≥ 2 casos) | `runtime.selectgo` | ordena por dirección, bloquea todos, elige al azar entre los listos |
| `select` + `default` (1 caso) | `runtime.selectnbrecv` / `selectnbsend` | operación no bloqueante directa |
| `defer f()` | *open-coded* (sin llamada) o `runtime.deferproc` / `deferreturn` | registra y ejecuta al salir |
| `mu.Lock()` | CAS en línea; si falla, `…Mutex.lockSlow` | semáforo + `gopark` |
| `x.(I)` | `runtime.typeAssert` / `assertE2I…` | conversión dinámica con caché de *itabs* |
| `panic`, `recover` | `runtime.gopanic`, `gorecover` | |
| `m[k] = v` | `runtime.mapassign…` | |
| `append` | `runtime.growslice` | |

### Cómo verlo tú mismo

- `go build -gcflags=-S` imprime el ensamblador de cada función; busca `CALL runtime.…`.
- `go tool objdump -s 'miFuncion' binario` sobre un binario ya compilado.
- `go tool compile -S fichero.go` (para un solo paquete sin dependencias).
- El código fuente en `$GOROOT/src/runtime` (`proc.go`, `chan.go`, `select.go`).

Cuidado: los *inlining* y otras optimizaciones cambian lo que se ve. Por eso el ejemplo usa `//go:noinline` y esta versión de Go; **no es API**, puede cambiar entre versiones.

## Qué hace el programa

1. Escribe en un directorio temporal un módulo con funciones mínimas (`Lanzar`, `Enviar`, `Recibir`, `Cerrar`, `Crear`, `Select2`, `SelectDefault`, `Diferir`, `Cerrojo`, `Rango`, `Interfaz`), cada una con `//go:noinline`.
2. Lo compila con `go build -gcflags=-S`.
3. Extrae del ensamblador las llamadas `CALL runtime.*` (y las del mutex) de cada función y las imprime, junto con una tabla de lectura.

```bash
go run .
```

## Resultados de referencia (Go 1.26.5, darwin/arm64)

```
  Cerrar                 runtime.closechan
  Cerrojo                internal/sync.(*Mutex).lockSlow, internal/sync.(*Mutex).unlockSlow
  Crear                  runtime.makechan
  Diferir                runtime.deferreturn, runtime.closechan
  Enviar                 runtime.chansend1
  Interfaz               runtime.typeAssert, runtime.panicnildottype
  Lanzar                 runtime.newproc
  Rango                  runtime.chanrecv2
  Recibir                runtime.chanrecv1
  Select2                runtime.selectgo
  SelectDefault          runtime.selectnbrecv
```

## Conclusiones

- **Cada construcción de concurrencia es una llamada concreta**: `go` → `newproc`, `<-` → `chanrecv1`, `select` → `selectgo`. No hay magia del lenguaje, hay una biblioteca de runtime.
- **`select` con `default` no pasa por `selectgo`**: es `selectnbrecv`, una operación no bloqueante sencilla. Un `select` con 2 casos sí llama a `selectgo` (con su coste de ordenar y bloquear todos los channels, ejercicio 6).
- **El `Lock()` del mutex solo llama a `lockSlow` en el camino lento:** la vía rápida (un CAS) queda **en línea** en el código del llamante; lo que se ve es el camino de contención.
- **`for v := range ch`** es `chanrecv2`: comprueba el segundo valor (`ok`) para saber si se cerró.
- Saber leer el ensamblador de esta forma te permite verificar suposiciones de rendimiento sin adivinar.

## Ejercicios propuestos

1. Añade al fragmento `append`, `map[k] = v`, `new(T)` que escape al heap y `[]byte(s)`; lista sus llamadas a `runtime.*`.
2. Compara `defer` en un bucle y fuera: ¿cuándo aparece `deferproc`?
3. Añade `sync.WaitGroup.Go` y `sync.Once.Do` y mira qué llama cada uno.
4. Compila con `-gcflags='-N -l'` (sin optimizaciones) y compara las llamadas.
