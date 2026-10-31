# Ejercicio 5: benchmarks rigurosos y `benchstat`

## Teoría

Un número de benchmark aislado no significa nada: **varía** entre ejecuciones (frecuencia de la CPU, temperatura, otros procesos, GC, planificador). Para afirmar "A es más rápido que B" hay que **repetir** y comparar **estadísticamente**.

### Escribir un buen benchmark

| Pauta | Cómo |
|--|--|
| **Medir solo lo que importa** | preparación fuera de la medición: `b.Loop()` la excluye automáticamente; con `b.N` usa `b.ResetTimer()` |
| **Que el compilador no elimine el trabajo** | `for b.Loop() { ... }` (Go 1.24+); con `b.N`, acumula el resultado en una variable global ("sumidero") |
| **Paralelo** | `b.RunParallel(func(pb *testing.PB){ for pb.Next() { ... } })`; **estado por goroutine** (generador aleatorio propio) para no medir la contención del propio benchmark |
| **Distintos escenarios** | `b.Run("lecturas=95%", ...)`: sub-benchmarks con nombre estable |
| **Asignaciones** | `b.ReportAllocs()` o `-benchmem` |
| **Escalabilidad** | `-cpu 1,2,4,8` |
| **Métricas propias** | `b.ReportMetric(valor, "unidad")` |
| **Datos realistas** | tamaño, distribución de claves y proporción lectura/escritura como en producción |

### Ejecutar con rigor

```bash
go test -bench . -run xxx -count 10 -cpu 8 > antes.txt      # 10 repeticiones
# ... cambias el código ...
go test -bench . -run xxx -count 10 -cpu 8 > despues.txt
benchstat antes.txt despues.txt                              # go install golang.org/x/perf/cmd/benchstat@latest
```

- `-run xxx` evita ejecutar los tests normales.
- **`-count ≥ 10`**: `benchstat` necesita varias muestras para estimar la dispersión.
- Mide con el equipo **tranquilo**: sin otras cargas, enchufado a la corriente, mismo estado térmico. Alterna "antes/después" si es posible.

### Leer la salida de `benchstat`

```
              │ antes.txt  │ despues.txt             │
              │   sec/op   │   sec/op     vs base    │
Almacen/…-8     71.22n ± 5%   31.30n ± 5%  -56.05% (p=0.000 n=10)
Almacen/…-8     62.42n ± 8%   61.62n ± 2%        ~ (p=0.631 n=10)
```

| Elemento | Significado |
|--|--|
| `71.22n ± 5%` | **mediana** y su dispersión (intervalo de confianza al 95 %) |
| `-56.05%` | cambio de la mediana |
| `p=0.000` | probabilidad de ver esta diferencia **por azar** si en realidad no hubiera ninguna (test U de Mann-Whitney). Menos de 0.05 ⇒ "significativa" |
| `~` | **sin diferencia significativa**: no se puede afirmar nada |
| `n=10` | muestras de cada lado |

**Dos reglas:** (1) un `p` bajo con efecto diminuto **no es una mejora real** (ver la comparación A/A abajo); (2) un `~` **no prueba** que sean iguales, solo que no hay evidencia suficiente.

## Qué contiene

- `almacen.go`: dos almacenes clave-valor: un `RWMutex` global y 64 fragmentos con `RWMutex`.
- `almacen_test.go`: tests de corrección (también con `-race`) y los benchmarks. La implementación se elige con `IMPL=rw|shard`, de modo que **el mismo benchmark con el mismo nombre** genera los dos archivos que `benchstat` compara.
- `comparar.sh`: ejecuta antes, después y *antes otra vez* (A/A) con 10 repeticiones y compara.

```bash
go test -race .
go install golang.org/x/perf/cmd/benchstat@latest
./comparar.sh                  # ≈ 2 minutos
```

## Resultados de referencia

**Antes (`RWMutex` global) frente a después (64 fragmentos), 8 CPUs, 10 repeticiones:**

```
                        │ antes.txt   │ despues.txt                │
                        │   sec/op    │   sec/op      vs base      │
Almacen/lecturas=50%-8    71.22n ±  5%   31.30n ±  5%  -56.05% (p=0.000 n=10)
Almacen/lecturas=95%-8    62.42n ±  8%   23.79n ±  8%  -61.89% (p=0.000 n=10)
Almacen/lecturas=100%-8   85.58n ± 13%   14.37n ± 16%  -83.21% (p=0.000 n=10)
geomean                   72.46n         22.03n        -69.59%
```

**El mismo código dos veces (A/A): ¿qué tanto ruido hay?**

```
Almacen/lecturas=50%-8    71.22n ±  5%   68.74n ± 2%   -3.49% (p=0.001 n=10)   <- "significativo" pero diminuto
Almacen/lecturas=95%-8    62.42n ±  8%   61.62n ± 2%        ~ (p=0.631 n=10)
Almacen/lecturas=100%-8   85.58n ± 13%   85.67n ± 4%        ~ (p=0.631 n=10)
```

**Trampas del microbenchmark** (`cuadrado(7)`, una función pura e inlinable):

```
BenchmarkTrampaViejoEstilo   0.35 ns/op   <- el compilador eliminó la llamada: mide un bucle vacío
BenchmarkConSumidero         0.33 ns/op   <- el resultado se guarda, pero la función se inlina y se calcula en ~1 ciclo
BenchmarkConLoop             2.3  ns/op   <- b.Loop mantiene la llamada (sin inlinarla)
```

## Conclusiones

- Fragmentar el almacén mejora **−56 % a −83 %** con p=0.000 en los tres escenarios: la mejora es real y grande, no ruido. La del 100 % de lecturas es la mayor (−83 %); probablemente porque los `RLock` del mutex global siguen contendiendo sobre su contador interno de lectores (no se verificó con un perfil).
- **La comparación A/A es imprescindible.** El mismo código salió con un **−3.49 % (p=0.001)** "significativo". Un cambio de menos del ~5 % en un equipo ruidoso **no se debe creer**, aunque `p` sea bajo. La dispersión (`± 13 %` en el escenario del 100 %) también avisa de que los resultados de ese caso son menos estables.
- **Mide el efecto, no solo el p-valor:** tamaño del cambio, dispersión y que tenga sentido con la teoría.
- **`b.Loop` no es "gratis":** puede impedir el inlining, por lo que una función minúscula parece más cara que dentro del programa real (2.3 ns frente a ~0.3 ns). Para funciones diminutas, el microbenchmark solo da el orden de magnitud: valida siempre en un benchmark más amplio o en un perfil del programa completo.
- En el diseño del benchmark paralelo, cada goroutine tiene su generador (`rand.New(PCG)`): si usara `rand.Intn` global o un mutex, mediría su propia contención.

## Ejercicios propuestos

1. Añade `-benchmem` y compara las asignaciones de ambas implementaciones.
2. Compara con `-cpu 1,2,4,8` en cada una y representa la escalabilidad.
3. Cambia `claves` a 64 y a 1 000 000: ¿cuándo deja de compensar fragmentar?
4. Compara `sync.Map` con las dos implementaciones usando `benchstat`.
