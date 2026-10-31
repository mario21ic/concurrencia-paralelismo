# Ejercicio 1: el planificador G-M-P, observado

## Teoría

El runtime de Go planifica **G** (goroutines) sobre **M** (hilos del SO) mediante **P** (procesadores lógicos, `GOMAXPROCS`). Sin un P, un M no puede ejecutar código Go.

| Estructura | Qué es |
|--|--|
| **`runnext`** (1 hueco por P) | la última goroutine creada/despertada por ese P: se ejecuta **la primera** |
| **cola local** (256 huecos por P) | FIFO de goroutines listas, sin cerrojo |
| **cola global** | desborde de las colas locales; se consulta cada 61 *ticks* y cuando un P se queda sin trabajo |
| **netpoller** | goroutines cuya E/S de red ya está lista |

### Cómo busca trabajo un P (`findRunnable`, simplificado)

1. `runnext` → cola local.
2. De vez en cuando (cada 61 ticks), la cola global (para que no pase hambre).
3. Cola global y netpoller.
4. **Robar (*work stealing*):** elige otro P al azar y le quita **la mitad de su cola local**.
5. Si no hay nada, el M se duerme (hilo "aparcado"); un M "girando" (*spinning*) busca antes de dormirse.

### Consecuencias observables

- **Orden:** con un solo P, `go A; go B; …; go H` ejecuta **`H A B C D E F G`**: `H` ocupó `runnext` y desplazó `G` al final de la cola; después, FIFO. **Nunca** dependas de este orden (no es parte de la especificación), pero explica por qué un `for { go f(i) }` parece ejecutar "primero la última".
- **Localidad:** una goroutine creada por otra suele ejecutarse en el **mismo P** (caché caliente); solo se reparte cuando otros Ps roban.
- **Desborde:** si la cola local se llena (256), la mitad pasa a la global (un cerrojo, más caro).

## Qué hace el programa

| Modo | Experimento |
|--|--|
| `-modo=orden` | 200 veces: `GOMAXPROCS=1`, crea 8 goroutines (A..H) y registra en qué orden empiezan |
| `-modo=reparto` | 4 000 goroutines creadas por **una** goroutine, con `GOMAXPROCS` 1, 4 y 8; usa la traza (`runtime/trace` + `x/exp/trace`) para contar en qué P empezó cada una |
| `-modo=cola` | desborda la cola local; se observa con `GODEBUG=schedtrace=1` (`runqueue=774` en la cola global) |

```bash
go run . -modo=orden
go run . -modo=reparto
GODEBUG=schedtrace=1 go run . -modo=cola 2>&1 | head
```

## Resultados de referencia (Apple M1, 8 núcleos, Go 1.26.5)

```
GOMAXPROCS=1, 8 goroutines creadas seguidas (A..H), main se bloquea:
  H A B C D E F G   (200 de 200 ejecuciones)

GOMAXPROCS=1: 4000 goroutines → 4010 inicios en 1 P    P0 4010
GOMAXPROCS=4: 4000 goroutines → 4024 inicios en 4 Ps   P0 1016  P1 1012  P2 980  P3 1016
GOMAXPROCS=8: 4000 goroutines → 4050 inicios en 8 Ps   P0 410 P1 509 P2 362 P3 591 P4 484 P5 583 P6 582 P7 529

SCHED 0ms: gomaxprocs=1 ... runqueue=774 [ 225 ]      ← 774 en la cola GLOBAL, 225 en la local del único P
```

## Conclusiones

- **El orden `H A B C D E F G` es determinista** (200 de 200), consecuencia directa de `runnext` + FIFO. Con más de un P deja de serlo, porque otros Ps roban.
- **El *work stealing* reparte casi uniformemente:** con 4 Ps cada uno ejecutó ≈ 25 % (980-1016 de 4 000); con 8, entre 362 y 591. Una sola goroutine creadora alimentó a todos los núcleos sin ninguna coordinación explícita.
- **Los inicios de ejecución superan ligeramente a las goroutines creadas** (4 010-4 050 > 4 000): además de las 4 000, se cuentan otras goroutines que corren durante la medición (main, la propia traza…); el exceso crece con el nº de Ps.
- **Con 1 P y miles de goroutines, la cola local (256) se desborda** y casi 800 pasan a la global (`runqueue=774`): por eso crear muchísimas goroutines de golpe es algo más caro que crearlas escalonadas.
- `GODEBUG=schedtrace=N` es la herramienta de primera mano: `idleprocs`, `threads`, `runqueue` (global) y el vector `[…]` (colas locales por P).

## Ejercicios propuestos

1. Repite `-modo=orden` con `GOMAXPROCS=2` y mide cuántos órdenes distintos aparecen en 1 000 ejecuciones.
2. Añade `runtime.Gosched()` tras cada `go` y explica el nuevo orden.
3. Usa `scheddetail=1` para ver los `G` de cada cola y verifica que `runnext` se vacía primero.
4. Mide el coste de crear 1 M de goroutines de golpe frente a crearlas en lotes de 200.
