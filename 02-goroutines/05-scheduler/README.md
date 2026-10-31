# Ejercicio 5: scheduler G-M-P

## Teoría

```
        cola global
            |
   +--------+--------+
   |        |        |
  [P0]     [P1]     [P2]        P = contexto de ejecución (GOMAXPROCS de ellos)
   |        |        |          cada P tiene una cola local de G listas
  (M0)     (M1)     (M2)        M = hilo de SO; ejecuta el código de la G que tiene su P
   G        G        G          G = goroutine
```

- **G:** goroutine (stack, estado, función).
- **M:** hilo del SO. Hay más M que P cuando algunos están bloqueados en llamadas al sistema.
- **P:** recursos para ejecutar Go (cola local, caché de memoria). Un M necesita un P para correr código Go.

Algoritmo simplificado de un P:
1. Ejecutar la siguiente G de su **cola local**.
2. Cada cierto tiempo, mirar la **cola global** (evita inanición).
3. Si está vacío, mirar el **netpoller** (I/O listo).
4. Si sigue vacío, **robar** la mitad de la cola de otro P (*work stealing*).

**Llamadas bloqueantes al sistema:** el M se bloquea con su G; el runtime le quita el P y se lo da a otro M para que los demás sigan. Con I/O de red no se bloquea ningún hilo: el *netpoller* aparca la G.

**Preemption:**
- *Cooperativa:* en llamadas a funciones, channels, `Gosched`, etc.
- *Asíncrona* (desde Go 1.14): una goroutine que ocupa el P más de ~10 ms recibe una señal y es expropiada, aunque esté en un bucle sin llamadas.

## Qué hace el programa

```bash
go run . -modo=preemption                 # 1 P y un bucle infinito: main sigue vivo
go run . -modo=gosched                    # 1 P y Gosched: alternancia
go build -o sched . && GODEBUG=schedtrace=300 ./sched -modo=carga
```

> Con `go run`, `GODEBUG=schedtrace` también afecta al propio compilador y mezcla líneas extra al inicio; compila con `go build` para ver solo las del programa.

## Resultado de referencia

```
-modo=preemption
main siguió ejecutando: el bucle fue expropiado

-modo=gosched
C1 A1 B1 C2 A2 B2 C3 A3 B3

-modo=carga (schedtrace cada 300ms; 64 goroutines de CPU, 8 P)
SCHED 318ms: gomaxprocs=8 idleprocs=0 threads=9 runqueue=35 [ 2 2 2 11 1 1 0 2 ] ...
SCHED 618ms: gomaxprocs=8 idleprocs=0 threads=9 runqueue=42 [ 1 1 1 1 1 2 5 2 ] ...
```

Cómo leer `schedtrace`:
- `gomaxprocs=8 idleprocs=0`: los 8 P están ocupados.
- `threads=9`: hilos de SO creados (M).
- `runqueue=35`: goroutines en la cola **global**.
- `[ 2 2 2 11 1 1 0 2 ]`: longitud de la cola **local** de cada P.
- `schedticks=[...]`: planificaciones por P; valores parejos indican reparto equilibrado.

## Conclusiones

- Con 1 P hay concurrencia sin paralelismo y el bucle infinito no congela al resto (preemption asíncrona).
- El orden de `gosched` es consistente por cómo el runtime reparte con un P (`C` primero porque la última G creada va a la ranura `runnext`), pero **no es un contrato**: no dependas de él.
- Con carga CPU y 64 G para 8 P, el trabajo se reparte entre colas locales y la global; los P se mantienen ocupados.

## Ejercicios propuestos

1. Ejecuta `-modo=carga` con `GOMAXPROCS=2` y observa las longitudes de las colas.
2. Genera un trace: `runtime/trace` + `go tool trace` y encuentra las goroutines en cada P.
3. Quita `runtime.GOMAXPROCS(1)` del modo `gosched` y compara la salida.
