# Ejercicio 4: el barbero dormilón

## Teoría

Otro problema clásico de Dijkstra: una barbería con **B barberos**, **S sillas** de espera y clientes que llegan al azar.

- Si no hay clientes, el barbero **duerme**.
- El cliente que llega **despierta** al barbero si duerme; si está ocupado, **se sienta** en una silla libre; si no hay ninguna, **se va**.

Es el modelo de un **sistema con cola acotada y pérdidas** (un *loss system*): cualquier servicio con **cola limitada y rechazo** —un servidor con pool de workers y cola de tamaño fijo, un balanceador, un `http.Server` con límite— se comporta igual. Se aplican resultados de **teoría de colas** (modelo M/M/c/K):

| Magnitud | Significado | Palanca |
|--|--|--|
| **Carga** ρ = λ·E[s] / B | nº medio de barberos que haría falta ocupados | más barberos (B) |
| **Probabilidad de pérdida** | clientes rechazados | más sillas (S) *o* más barberos |
| **Espera media** | latencia en cola | menos sillas / más barberos |
| **Ley de Little** | L = λ·W (clientes en el sistema = tasa × tiempo medio) | relaciona cola y latencia |

Si ρ ≥ 1 la cola crece sin límite; una cola **acotada** convierte ese crecimiento en **rechazos**: eso es *backpressure*.

### Compromiso fundamental: sillas

| Más sillas | Menos sillas |
|--|--|
| menos clientes perdidos (mejor *throughput*) | respuesta más rápida para los que entran |
| **más latencia** (la espera sube) y más memoria | más rechazos, pero rápido ("fallar pronto") |

Una cola enorme **no resuelve** la sobrecarga: solo la esconde y empeora la latencia (*bufferbloat*). Por eso en servicios reales suele ser mejor una cola **pequeña** + rechazo explícito (módulo 8, ejercicio *worker pool* y ejercicio 10 de este módulo).

### Cómo se traduce a Go

| Barbería | Go |
|--|--|
| sala de espera con S sillas | `chan *cliente` con buffer S |
| barbero dormido | goroutine bloqueada en `<-sala` (sin coste de CPU) |
| despertar al barbero | enviar al channel |
| "no hay silla, me voy" | `select { case sala <- c: … default: perdido }` |
| cliente esperando su corte | `<-c.hecho` (channel por cliente que se cierra al acabar) |

Con **0 sillas** el channel no tiene buffer: el envío solo triunfa si hay un barbero **ya esperando en la recepción** (despierto y libre). Es la semántica exacta de "solo entro si el barbero duerme".

## Qué hace el programa

`barberia.go` implementa `Barberia` (`Abrir`, `Llega`, `Cerrar`) con métricas. `main.go` lanza 300 clientes con llegadas exponenciales (media 4 ms) y cortes de 6 ms, con distintas combinaciones de barberos y sillas.

```bash
go run .
go test -race -v .        # con testing/synctest: tiempo virtual, resultados exactos
```

Los tests usan **`synctest`**: el reloj virtual hace que las llegadas y cortes sean **deterministas**, así que se pueden afirmar cifras exactas (p. ej. con 1 barbero, 2 sillas y 10 llegadas seguidas se atienden exactamente 3 y se pierden 7).

## Resultados de referencia

```
300 clientes, llegada media 4ms, corte 6ms (carga 1.5 con un barbero)

  barberos  sillas  perdidos  % perdido  espera media   espera máx
  1         0       179       59.7       0s             0s
  1         2       114       38.0       8.4ms          14ms
  1         10      104       34.7       56ms           66ms
  2         0       84        28.0       0s             0s
  2         2       16        5.3        1.4ms          7ms
  2         10      0         0.0        3.2ms          20ms
  3         2       3         1.0        200µs          4ms
```

## Conclusiones

- **Con un barbero sobrecargado (ρ = 1.5), añadir sillas casi no ayuda:** de 2 a 10 sillas se pierden 114 → 104 clientes (38 % → 35 %), pero la **espera media se multiplica por 7** (8 ms → 56 ms). Una cola grande solo añade latencia: el sistema sigue sin dar abasto.
- **Añadir un barbero sí cambia la situación:** con 2 barberos y 2 sillas se pierde el 5 %; con 3 barberos, el 1 %.
- **Las sillas funcionan como amortiguador de ráfagas** cuando la carga media es < 1 (2 barberos): de 0 a 2 sillas se pasa del 28 % al 5 % de pérdidas, con espera media de solo 1.4 ms.
- **0 sillas = sin espera pero muchas pérdidas** (la única forma de entrar es coincidir con un barbero libre).
- El código no tiene `sync.Mutex` en la ruta crítica: la **sincronización está en los channels**, y el "dormir" es gratis (goroutine bloqueada).

## Ejercicios propuestos

1. Cambia a **cola de prioridad** (clientes VIP): ¿qué pasa con la espera máxima del resto?
2. Añade **impaciencia**: el cliente abandona la silla si espera más de `T` (`context.WithTimeout`). ¿Cuánto cambian las pérdidas?
3. Haz que el barbero cobre y haya **un solo cajero**: segundo recurso compartido; mide el nuevo cuello de botella.
4. Compara con la fórmula de Erlang-B para `S = 0` y verifica el % perdido simulado.
