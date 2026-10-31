# Ejercicio 5: cola acotada y ring buffers

## Teoría

Una **cola acotada** (*bounded queue*) es la pieza base de casi todo sistema concurrente: pasa trabajo de productores a consumidores y, al estar **limitada**, aplica *backpressure* (el productor se frena o falla cuando se llena). Tres formas de construirla:

| Estructura | Cómo sincroniza | Bloquea | Cuándo |
|--|--|--|--|
| **`chan T` con buffer** | cerrojo interno del runtime + colas de espera | sí (duerme la goroutine) | **la opción por defecto**; integra `select`, `close`, `range` |
| **`Cola[T]`** (este ejercicio) | `sync.Mutex` + dos `sync.Cond` sobre un buffer circular | sí | cuando necesitas **algo que el channel no da**: `Len`, `TryPut`, cancelar con `ctx` + `Close` con semántica propia, vaciar, prioridades |
| **Ring buffer sin cerrojos** | atomics (contadores o secuencias por celda) | **no** (gira o falla: `TryPush`/`TryPop`) | rutas de latencia ultrabaja, productores/consumidores en hilos dedicados; coste: CPU al girar |

### Buffer circular

Un array de capacidad `N` con dos contadores que **solo crecen**: `cola` (escritura) y `cabeza` (lectura). La posición es `contador % N`; con `N` potencia de 2 se calcula `contador & (N-1)`. **Lleno** si `cola − cabeza == N`; **vacío** si `cola == cabeza`. Los contadores `uint64` no se desbordan en la práctica.

### `Cola[T]`: bloqueo con `sync.Cond` y cancelación

- Dos condiciones (`noVacia`, `noLlena`) sobre el mismo mutex; **siempre** `for cond { Wait() }` (nunca `if`: despertares espurios y robos de turno).
- `sync.Cond` **no es cancelable**. El truco idiomático: `context.AfterFunc(ctx, …)` que toma el mutex y hace `Broadcast`, y el bucle reevalúa `ctx.Err()`. Tomar el mutex dentro de `AfterFunc` es imprescindible; si no, la señal se puede perder entre la comprobación y el `Wait`.
- `Close()` despierta a todos; `Get` entrega lo pendiente y después `ErrCerrada` (igual que un channel).

### Ring SPSC (un productor, un consumidor)

Algoritmo de **Lamport**: el productor es el único que escribe `cola`; el consumidor, el único que escribe `cabeza`. Sin CAS: basta `Store` atómico para **publicar** (el dato se escribe antes; el `Store` establece *happens-before*). Dos optimizaciones, medidas aquí:

1. **Contadores en líneas de caché distintas** (`cpu.CacheLinePad`; módulo 13: *false sharing*).
2. **Copia local del contador ajeno**: el productor solo relee `cabeza` cuando, según su copia, el anillo *parece* lleno (y el consumidor igual con `cola`). Sin esto, la línea de `cabeza` viaja entre núcleos **en cada inserción**.

### Ring MPMC acotado (Vyukov)

Cada celda tiene un **número de secuencia** que indica de quién es el turno. Un productor reclama una posición con un **CAS** sobre `cola` y comprueba que la celda esté libre para *su vuelta* (`sec == pos`); escribe y publica con `sec = pos+1`. El consumidor espera `sec == pos+1`, lee y libera con `sec = pos + N`. No hay cerrojo global: solo disputan el CAS del contador.

### Elegir

- ¿Necesitas `select`, `range`, `close`? → **channel**.
- ¿Muy alta frecuencia con un solo productor y consumidor, y puedes dedicar un hilo? → **ring SPSC**.
- ¿Muchos productores/consumidores y no quieres dormir goroutines? → **ring MPMC** (acepta girar; mide CPU total).
- ¿Quieres sincronizar con otras condiciones (`ctx`, `Len`, prioridades)? → **`Cola[T]`**.

## Qué hace el programa

`cola.go` (`Cola[T]`), `ring.go` (`RingSPSC`, `RingMPMC`) y `main.go`, que mide cada una con 4 M de elementos y buffer de 1024, 1×1 y 4×4. Los tests comprueban orden FIFO, cancelación, `Close`, que **cada elemento sale exactamente una vez** con 4×4 (suma de control) y ausencia de fugas (`goleak`).

```bash
go run .
go test -race -v .
```

## Resultados de referencia (Apple M1, 8 núcleos, Go 1.26.5; ns por elemento)

```
1 productor + 1 consumidor:
  chan int                   34.1
  Cola (mutex+cond)         174.7
  RingSPSC (sin cerrojos)    12.8     (50.0 sin la copia local del contador ajeno)
  RingMPMC (Vyukov)          15.2

4 productores + 4 consumidores:
  chan int                   44.7
  Cola (mutex+cond)         349.3
  RingMPMC (Vyukov)         154.9
```

## Conclusiones

- **El channel del runtime es muy competitivo** (34 ns) y **5× más rápido que mi `Cola` con `Cond`** (175 ns): despertar goroutines con `Cond` y tomar dos veces el mutex por elemento cuesta. Reimplementar un channel solo se justifica por las funciones extra, no por rendimiento.
- **El ring SPSC es 2.7× más rápido que el channel** (12.8 frente a 34 ns) porque no duerme goroutines ni usa cerrojos; **pero la primera versión, sin copia local del contador del otro extremo, tardaba 50 ns** (más lenta que el channel): releer el contador ajeno en cada operación hace rebotar su línea de caché. Una optimización de ~15 líneas dio 3.9×.
- **Con 4+4 el ring MPMC pierde frente al channel:** 155 ns frente a 45 ns (3.5× más lento). Con muchos productores el CAS sobre un único contador es el cuello de botella y los hilos que giran compiten con los que trabajan. **Sin cerrojos no significa más rápido**; el channel del runtime duerme a quien no puede avanzar y deja la CPU a quien sí.
- Los rings **giran** (aquí con `runtime.Gosched`): en un escenario real conviene que el consumidor tenga un hilo propio o combinar con un *backoff* (girar poco, luego dormir).
- Todo `go test -race` pasa, y el `goleak` confirma que `Cola` no deja goroutines tras `Close`/cancelación.

## Ejercicios propuestos

1. Añade a `Cola[T]` un `PutAll`/`GetBatch` que mueva varios elementos por cada toma del mutex y compara con el batching del módulo 13.
2. Añade *backoff* progresivo (girar → `Gosched` → dormir con `time.Sleep`) a los rings y mide el uso de CPU.
3. Implementa `TryPush` en el SPSC para **lotes** (reservar `k` posiciones y publicar una sola vez).
4. Haz que el MPMC tenga `Close`: los consumidores salen al vaciarse. Cuidado con la carrera entre "vacío" y "cerrado".
