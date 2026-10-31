# Módulo 13. Rendimiento y paralelismo real

**Objetivo:** convertir "usar goroutines" en **aceleración real y medida**: entender qué limita la escalabilidad (hardware incluido), eliminar la contención oculta, amortizar costes fijos, comparar versiones con rigor estadístico y aplicar paralelismo de datos con criterio.

## Ejercicios

| # | Carpeta | Tema |
|---|---------|------|
| 1 | [01-escalabilidad](01-escalabilidad) | Speedup y eficiencia; tres cargas con tres techos distintos |
| 2 | [02-false-sharing](02-false-sharing) | Líneas de caché, relleno y por qué "64 bytes" no es universal |
| 3 | [03-contador-fragmentado](03-contador-fragmentado) | Reducir contención: atómico, fragmentos, afinidad, acumulación local |
| 4 | [04-batching](04-batching) | Amortizar costes fijos (channels, locks) y preasignar |
| 5 | [05-benchmarks-y-benchstat](05-benchmarks-y-benchstat) | `b.Loop`, `RunParallel`, `-count`, `benchstat`, comparación A/A |
| 6 | [06-map-reduce](06-map-reduce) | Particionar datos, estado local, fusión paralela |
| 7 | [07-merge-sort-paralelo](07-merge-sort-paralelo) | Divide y vencerás: umbral y mezcla paralela |

```bash
cd 13-rendimiento/03-contador-fragmentado && go test -bench . -run xxx -cpu 1,2,4,8
```

> **Hardware de las mediciones:** Apple M1, 8 núcleos (4 de rendimiento + 4 de eficiencia), **líneas de caché de 128 bytes**, Go 1.26.5. Casi todos los techos y proporciones de este módulo dependen de eso; en tu máquina cambiarán. **Lo reutilizable es el método.**

## Resumen teórico

### 1. Primero decide qué limita tu programa

| Si el cuello de botella es… | Más goroutines… | Qué ayuda |
|--|--|--|
| **Cálculo** | ayudan hasta el nº de núcleos *equivalentes* | repartir el trabajo; menos instrucciones |
| **Ancho de banda de memoria** | **no ayudan** más allá de 3-4 hilos | procesar por bloques que quepan en caché; datos más compactos |
| **Asignaciones / GC** | ayudan poco | asignar menos (preasignar, reutilizar, `sync.Pool`, lotes) |
| **Un lock / un atómico compartido** | **empeoran** (escalado negativo) | fragmentar, estado local, no compartir |
| **E/S / red** | ayudan mucho (más que `NumCPU`) | concurrencia acotada por el recurso externo |

El **perfil** (módulo 12) dice cuál es el tuyo; no lo supongas.

### 2. Medir la escalabilidad

- **Speedup** `S(k) = T(1)/T(k)`; **eficiencia** `S(k)/k`.
- **Amdahl:** `S(∞) = 1/(1 − p)`. **Gustafson:** si el problema crece con los núcleos, el speedup escala mejor.
- **Núcleos heterogéneos y SMT** rompen el supuesto de núcleos iguales: un modelo bien formulado aplicado a un hardware que no lo cumple da conclusiones equivocadas (ejercicio 1).
- Escalado **fuerte** (mismo trabajo total) frente a **débil** (mismo trabajo por worker).

### 3. Contención: la principal causa de mal escalado

| Fuente | Síntoma | Remedio |
|--|--|--|
| Mutex global | espera de goroutines | reducir la sección crítica; fragmentar |
| **Atómico global** | las líneas de caché rebotan entre núcleos | fragmentar con relleno; acumular local |
| ***False sharing*** | datos independientes en la misma línea | **relleno** hasta una línea de caché (`cpu.CacheLinePad`) |
| Fragmentos accedidos al azar | las líneas **migran** entre núcleos | **afinidad**: fragmento estable por worker |
| Asignador/GC | contención del runtime | asignar menos |

**La mejor sincronización es la que no existe:** estado local por goroutine + combinar al final.

### 4. Amortizar: batching y preasignación

`coste por elemento ≈ trabajo + coste_fijo / N`. Aplica a channels, locks, E/S y asignaciones. Contrapartida: latencia (el primer elemento espera al lote) ⇒ lotes por **tamaño o tiempo**. Rendimientos decrecientes pasado cierto tamaño.

### 5. Paralelismo de datos

1. **Particionar** en trozos contiguos.
2. **Estado local** por worker.
3. **Combinar** (y paralelizar la combinación si pesa).
4. Un **umbral** por debajo del cual no se paraleliza (el coste de coordinar > el ahorro).
5. Cuidado con **duplicar estado** (cada worker con su copia del mapa grande) y con lo que sigue siendo **secuencial** (la última fusión, la última mezcla, la copia).

### 6. Benchmarks que no mienten

- `for b.Loop()` (Go 1.24+), preparación fuera de la medición, estado por goroutine en `RunParallel`.
- `-count 10` + `benchstat`; compara siempre también **A/A** para conocer tu ruido.
- Fíjate en el **tamaño del efecto** y la **dispersión**, no solo en `p`.
- `-cpu 1,2,4,8` para ver la **escalabilidad**, no solo un número.
- Un microbenchmark da el **orden de magnitud**; valida con un benchmark amplio o con perfil del programa real.

### Reglas de oro

1. **Mide antes y después**, con la misma carga y estadística.
2. **Elimina el estado compartido** antes de optimizar su sincronización.
3. **Entiende el hardware** (núcleos, cachés, líneas de 64/128 B, ancho de banda de memoria).
4. **Más goroutines ≠ más rápido**: busca el techo y respétalo.
5. **Cada optimización tiene un precio** (memoria, latencia de lectura, complejidad): anótalo.
6. La biblioteca estándar suele ser una referencia difícil de batir (ejercicio 7); compárate con ella.

## Hallazgos del módulo (aprendidos midiendo)

- **El techo depende del recurso limitante:** 5.3× (cálculo), 2.9× (memoria), 2.7× (asignaciones) en la **misma** máquina.
- **El *false sharing* costó hasta 3.4×** sin un solo lock, y **"64 bytes" no bastó** en un chip con líneas de 128.
- **Un atómico compartido empeora al añadir núcleos** (7.5 → 34.6 ns) y un mutex todavía más (15 → 103 ns). Fragmentar **al azar no escala** (las líneas migran); el fragmento **afín** sí: 1.46 ns (**7×** mejor).
- **Batching:** ~32× (channel, lote 256) y hasta ~71× reciclando lotes; preasignar cadenas: 13.8×.
- **`benchstat`:** −56 % a −83 % real y significativo; y el **mismo código dos veces** salió con un −3.5 % "significativo": ruido.
- **Map-reduce:** el mapa compartido con mutex fue **13× más lento** que el secuencial; con estado local 3.5-4.1×; con 2 M de claves, solo 1.2-1.5×.
- **Merge sort paralelo:** meseta de 8 191 a 7 goroutines (≈ 3.7×), mal con 262 143 goroutines (2.56×) o con 1 (1.85×); mezcla paralela: 4.58×, **4.2× más rápido que `slices.Sort`**.
- Errores propios que las pruebas destaparon: un `locales[0]` con entrada vacía (ejercicio 6), y una "trampa de benchmark" que no se disparaba con `suma(1000)` y hubo que rediseñar con una función inlinable (ejercicio 5).

## Preguntas de repaso

1. ¿Por qué ocho núcleos pueden dar solo 5.3× en una carga de cálculo puro?
2. ¿Qué es el *false sharing* y cómo se detecta sin un profiler de bajo nivel?
3. ¿Por qué un atómico compartido empeora al añadir núcleos si no hay locks?
4. ¿Qué diferencia hay entre fragmentar al azar y con afinidad? ¿Por qué solo uno escala?
5. ¿Qué precio paga un contador fragmentado?
6. Si `coste/elemento ≈ trabajo + fijo/N`, ¿por qué no usar siempre N enorme?
7. ¿Qué te dice un `~` en `benchstat`? ¿Y un `p=0.001` con un −3 %?
8. ¿Por qué un mapa compartido con mutex puede ser más lento que el código secuencial?
9. ¿Qué papel juega el umbral en un algoritmo de divide y vencerás paralelo?
10. ¿En qué parte aplica Amdahl a la ordenación por mezcla paralela?

## Lecturas

- [Amdahl's law](https://en.wikipedia.org/wiki/Amdahl%27s_law) y [Gustafson's law](https://en.wikipedia.org/wiki/Gustafson%27s_law)
- *What Every Programmer Should Know About Memory* — Ulrich Drepper (cachés, coherencia, ancho de banda)
- [`golang.org/x/perf/cmd/benchstat`](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat) y [`testing.B.Loop`](https://pkg.go.dev/testing#B.Loop)
- [`golang.org/x/sys/cpu`](https://pkg.go.dev/golang.org/x/sys/cpu) (`CacheLinePad`)
- *The Art of Multiprocessor Programming* — Herlihy & Shavit (contadores, contención, estructuras escalables)
- Módulos anteriores: 1 (Amdahl y CPU/E/S), 6-7 (mutex y atómicos), 8 (pools, batching), 12 (perfiles y GC).
