# Caso 8.1 — Medir el speedup (y distinguir concurrencia de paralelismo)  ·  `básico`

> **Problema:** Tengo 256 bloques de cálculo puro (sin E/S ni memoria compartida). ¿Cuánto más rápido termina con 2, 4, 8, 16 workers? ¿Y qué pasa con las **mismas goroutines** si el runtime solo puede usar un núcleo?

## Qué features usa y por qué

| Feature de Go | Papel en este caso |
|--|--|
| `sync.WaitGroup.Go` | lanzar los `k` workers y esperar |
| Contador `atomic.Int64` | reparto **dinámico**: cada worker coge el siguiente bloque libre |
| Acumulador local por worker | `parciales[w]`: nada compartido que proteger; el XOR final no depende del orden |
| `runtime.GOMAXPROCS(n)` | limitar cuántos hilos ejecutan Go a la vez: separa «hay goroutines» de «hay paralelismo» |

Las dos métricas que debes calcular **siempre** que paralelices:

```
aceleración (speedup) = T(1 worker) / T(k workers)
eficiencia            = aceleración / núcleos realmente usables      (100 % = reparto perfecto)
```

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ Para decidir con datos si merece la pena paralelizar y con cuántos workers (empieza con `GOMAXPROCS`).
- ✅ Para entender que **concurrencia ≠ paralelismo**: con `GOMAXPROCS=1` las 8 goroutines están «en curso» a la vez pero se turnan en un único núcleo.
- ❌ No sirve para decidir en trabajo de E/S (red, disco): ahí manda la latencia, no los núcleos (ver [7.1](../../07-io-vs-cpu/1-basico-cuantos-workers)).

## Cómo ejecutarlo

```bash
go run .
go test -race -v .
```

## Qué observar (salida real, Apple M1, Go 1.26.5)

```
GOMAXPROCS=8. 256 bloques de 1000000 iteraciones de CPU pura (mejor de 3).

workers   tiempo        aceleración   eficiencia
1         513ms         1.00×         100 %
2         268ms         1.91×         96 %
4         139ms         3.69×         92 %
8         89ms          5.73×         72 %
16        97ms          5.30×         66 %
32        90ms          5.67×         71 %

8 workers con GOMAXPROCS=1: 501ms (1.02× frente a 1 worker) → concurrencia SIN paralelismo
```

Cómo leerlo:

- Hasta 4 workers la eficiencia es ≈ 92-96 %: el trabajo es CPU puro e independiente, casi el caso ideal.
- Con 8 workers **no** se llega a 8×: 5.73×. El M1 tiene 4 núcleos de rendimiento y 4 de eficiencia, que no rinden igual; con reparto dinámico los lentos terminan menos bloques, pero ya no se escala linealmente. (Además había una videollamada abierta durante la medición: las cifras varían unos puntos de una ejecución a otra.)
- **16 y 32 workers no mejoran a 8**: por encima del número de núcleos solo compiten por ellos (y cuestan algo de cambio de contexto).
- La última línea es la clave conceptual: 8 goroutines, mismo código, `GOMAXPROCS=1` → **1.02×**. Sin núcleos libres no hay paralelismo, aunque haya concurrencia.

## Errores típicos

- Decir «paralelicé con 8 goroutines» sin medir el speedup real.
- Poner muchos más workers que núcleos para trabajo CPU puro «por si acaso».
- Comparar contra `k=1` *con el overhead del reparto* en vez de contra la versión secuencial pura (aquí `k=1` tiene el contador atómico, pero es despreciable frente a 1 M de iteraciones por bloque).
- Dar por hecho que `NumCPU` núcleos son iguales entre sí (portátiles con núcleos P/E, máquinas virtuales, contenedores con cuota).

## Profundiza en el curso

- [concurrencia frente a paralelismo](../../../01-fundamentos/01-concurrencia-vs-paralelismo)
- [la ley de Amdahl](../../../01-fundamentos/05-amdahl) (siguiente caso: [8.2](../2-intermedio-amdahl))
- [escalabilidad, speedup y eficiencia](../../../13-rendimiento/01-escalabilidad)
- [`GOMAXPROCS` y `NumCPU`](../../../02-goroutines/04-runtime-info)
- [el planificador G-M-P](../../../14-internals/01-gmp-colas)

## Prueba tú

1. Ejecuta con `GOMAXPROCS=4 go run .` y compara la columna de 8 workers.
2. Cambia `Gasta` por una versión que reserve memoria en cada iteración (`make([]byte, 64)`): ¿sigue escalando igual? (adelanto de [8.3](../3-avanzado-memoria-y-cache)).
3. Reduce los bloques a 8 (`nBloques = 8`): ¿qué ocurre con 16 y 32 workers y por qué?
