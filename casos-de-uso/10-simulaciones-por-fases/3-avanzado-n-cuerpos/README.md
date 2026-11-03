# Caso 10.3 — N cuerpos: workers persistentes y barrera cíclica  ·  `avanzado`

> **Problema:** N partículas se atraen por gravedad. Cada paso tiene **dos fases dependientes**: calcular las aceleraciones (lee las posiciones de todas) y mover (cada worker escribe las suyas). ¿Lanzo goroutines nuevas en cada fase o dejo workers vivos que se sincronizan con una barrera?

## Qué features usa y por qué

| Pieza | Papel |
|--|--|
| **Estructura de arrays** (`x, y, vx, vy, m, ax, ay []float64`) | recorrido secuencial en memoria, mejor para la caché que un slice de structs |
| **Fase 1** `aceleraciones` | solo **lee** `x, y, m`; escribe `ax, ay` de sus cuerpos |
| **Fase 2** `mover` | escribe `x, y, vx, vy` solo de sus cuerpos |
| **Barrera entre fases** | nadie mueve un cuerpo mientras otro aún lee posiciones viejas; y el paso siguiente necesita las posiciones nuevas de todos |
| `PorFase` | `k` goroutines nuevas y `WaitGroup` en cada fase (2 veces por paso) |
| `Persistentes` + `Barrera` | `k` goroutines que viven toda la simulación; una **barrera cíclica** (`sync.Mutex` + `sync.Cond` + contador de generación) las sincroniza |

La barrera usa un **contador de generación** (`barrera.go`): sin él, una goroutine rápida que da la vuelta y vuelve a llamar a `Esperar` podría despertarse con el `Broadcast` de la ronda anterior o colarse en ella.

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ **`PorFase`** (lanzar y esperar) como opción por defecto: es la más simple y las goroutines son baratas.
- ✅ **Workers persistentes + barrera** cuando cada worker mantiene **estado propio** caro de recrear (buffers, generadores, conexiones) o cuando el número de fases es enorme.
- ❌ Una barrera hecha a mano es código delicado (condiciones de carrera, `Wait` en bucle, generaciones). Si `WaitGroup` basta, úsalo.
- ❌ Pasos muy cortos (decenas de microsegundos): ninguna de las dos formas compensa (ver `N = 64`).

## Cómo ejecutarlo

```bash
go run .
go test -race -v .
```

## Qué observar (salida real, Apple M1, Go 1.26.5)

```
GOMAXPROCS=8, k=8. N cuerpos, mejor de 3. Aceleración frente al secuencial.

N        pasos    secuencial     goroutines por fase      workers + barrera       
64       20000    109.1ms        161.7ms (0.7×)           143.7ms (0.8×)          
256      2000     172.5ms        117ms (1.5×)             111.8ms (1.5×)          
1024     200      271.8ms        90ms (3.0×)              104.3ms (2.6×)          
4096     20       449.5ms        141.3ms (3.2×)           130.2ms (3.5×)          
```

- **Con N pequeño el paralelismo pierde** (0.7-0.8× con 64 cuerpos): un paso sale en ≈ 5 µs secuencial, y sincronizar 8 workers dos veces por paso cuesta más que eso.
- A partir de 256 cuerpos ya gana, y con 4096 llega a **3.2-3.5×**. El cálculo es O(N²), así que el trabajo por paso crece mucho más deprisa que el coste de sincronizar.
- **Las dos formas de sincronizar rinden casi igual** (1.5× y 1.5×, 3.0× y 2.6×, 3.2× y 3.5×): no hay un ganador claro. Esperaba que los workers persistentes ganaran al ahorrarse crear goroutines, pero crear una goroutine es tan barato que no se nota, y la barrera con `Cond` tiene su propio coste. Por eso `PorFase` es la opción por defecto.
- Los tests comprueban que **todas las versiones dan exactamente el mismo resultado** (cada cuerpo suma las fuerzas en el mismo orden), que la barrera es correcta durante 200 rondas con 6 workers (con `-race`), que bloquea hasta que llega el último (con `synctest`) y que se **conserva el momento lineal total** (acción y reacción), un invariante físico que detectaría un reparto defectuoso.

## Errores típicos

- Una sola barrera por paso: si un worker empieza a escribir posiciones mientras otro aún calcula aceleraciones, lee posiciones a medias (resultado erróneo no determinista).
- Barrera de un solo uso en un bucle: la segunda ronda se cuelga o se salta.
- Usar `if` en lugar de `for` alrededor de `cond.Wait()`: despertares espurios.
- Que un worker termine antes (por un `return` o un pánico) y los demás esperen para siempre en la barrera: la barrera exige que lleguen **todos**.

## Profundiza en el curso

- [`sync.Cond`](../../../06-sync/04-cond): la base de la barrera
- [WaitGroup](../../../02-goroutines/03-waitgroup): barrera de un solo uso
- [testing con `synctest`](../../../11-testing)
- [false sharing](../../../13-rendimiento/02-false-sharing): relevante si los workers escribieran arrays contiguos
- [el caso anterior: Juego de la Vida](../2-intermedio-juego-de-la-vida)

## Prueba tú

1. Añade `k=4` y `k=16` a la comparación: ¿cuál es el mejor para `N = 1024`?
2. Convierte la barrera en un `chan` de «llegadas»: ¿es más sencilla o más cara?
3. Haz que el paso 100 imprima el momento total: ¿se conserva con la precisión que esperas?
