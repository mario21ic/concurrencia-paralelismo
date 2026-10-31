# Ejercicio 7: GC, `GOGC`, `GOMEMLIMIT` y concurrencia

## Teoría

El recolector de basura de Go es **concurrente**: marca objetos vivos **mientras** tu código sigue ejecutándose, con pausas de *stop-the-world* muy cortas (cientos de µs). Pero no es gratis: usa **CPU** (hasta el 25 % de los `P` de forma dedicada, más *asistencias* a las goroutines que asignan demasiado rápido) y **memoria** (necesita espacio extra mientras la aplicación sigue asignando).

### Los dos mandos

| Variable | Qué controla | Por defecto |
|--|--|--|
| **`GOGC`** (o `debug.SetGCPercent`) | cuánto puede crecer el heap antes del siguiente ciclo: **objetivo = vivo × (1 + GOGC/100)** | 100 |
| **`GOMEMLIMIT`** (o `debug.SetMemoryLimit`) | **límite blando** de memoria total; el GC se vuelve más agresivo al acercarse | sin límite |

- `GOGC` alto ⇒ **menos GC** y más CPU para tu código, **más memoria**.
- `GOGC` bajo ⇒ **menos memoria**, más CPU en GC.
- `GOGC=off` + `GOMEMLIMIT`: "usa toda la memoria que te doy y recolecta solo al llegar". Es un límite **blando**: el runtime limita el tiempo de GC (≈ 50 % de la CPU) para evitar la espiral de la muerte, pero puede superarlo.
- En **contenedores**: fija `GOMEMLIMIT` algo por debajo del límite del contenedor (p. ej. 90 %) para que el GC actúe antes del OOM-kill.

### Nuevo en Go 1.26: Green Tea

El recolector **Green Tea** es el nuevo algoritmo de marcado por defecto (mejor localidad de memoria al escanear objetos pequeños). Se puede volver al clásico con `GOEXPERIMENT=nogreenteagc` al compilar (válido en esta versión; podría desaparecer en el futuro).

### Leer una línea de `gctrace`

```
GODEBUG=gctrace=1 ./programa
gc 5 @0.817s 12%: 0.57+168+0.057 ms clock, ... 886->1164->332 MB, 1162 MB goal, ..., 8 P
```

| Campo | Significado |
|--|--|
| `gc 5` | ciclo nº 5 |
| `@0.817s` | momento desde el arranque |
| `12%` | % de CPU total usado por el GC hasta ahora |
| `0.57+168+0.057 ms clock` | parada inicial + marcado **concurrente** + parada final |
| `886->1164->332 MB` | heap **al empezar** → **al terminar** el marcado → **vivo** tras marcar |
| `1162 MB goal` | objetivo del heap para este ciclo |

### El heap "vivo" incluye lo asignado durante el marcado

Todo lo que se asigna mientras el GC marca se trata como vivo **en ese ciclo** (se conserva hasta el siguiente). Si asignas a gran velocidad, el "vivo" que mide el GC (232-332 MB en la traza de este ejercicio con GOGC=400) es **muy superior** al conjunto residente real (~58 MB), y como el objetivo es `vivo × (1 + GOGC/100)`, **el heap se infla**.

## Qué hace el programa

8 workers asignan sin parar (~2 GB/s de basura de objetos con puntero) sobre un conjunto residente constante de **~58 MB** (150 000 objetos por worker). Se mide el rendimiento (millones de objetos por segundo), los ciclos de GC, la fracción de CPU del GC (`MemStats.GCCPUFraction`) y el pico de `HeapInuse` (muestreado cada 5 ms).

```bash
go run . -gogc 100
GODEBUG=gctrace=1 go run . -gogc 400 -dur 1s
./comparar.sh        # tabla comparativa (compila también la variante nogreenteagc)
```

## Resultados de referencia (8 núcleos, 2 s por ejecución, rangos de 3 ejecuciones)

| Configuración | Rendimiento (M obj/s) | Ciclos de GC | CPU del GC | Pico de heap |
|--|--|--|--|--|
| `GOGC=25` | 14 – 18 | 88 – 111 | 43 – 49 % | 90 – 117 MiB |
| `GOGC=100` | 36 – 48 | 42 – 55 | 36 – 41 % | 182 – 191 MiB |
| `GOGC=400` | 42 – 88 | 6 – 8 | 15 – 17 % | **1.5 – 2.3 GiB** |
| `GOGC=off` + `GOMEMLIMIT=400MiB` | 8.7 – 9.0 | 61 – 62 | 39 % | 385 MiB |
| `GOGC=off` + `GOMEMLIMIT=120MiB` | 7.1 – 7.3 | 119 – 123 | 47 % | 115 MiB |

**Green Tea frente a GC clásico** (`GOGC=100`, 3 ejecuciones cada uno):

```
green tea : 46.7 – 52.2 M obj/s | GC 37.8 – 39.3 % de la CPU | 53 – 59 ciclos
clásico   : 42.6 – 48.5 M obj/s | GC 40.5 – 42.0 % de la CPU | 46 – 52 ciclos
```

**`gctrace` con `GOGC=400`:** `886->1164->332 MB, 1162 MB goal`: el heap llegó a 1.1 GB porque el "vivo" medido (332 MB) es ~6× el residente real (58 MB).

## Conclusiones

- **`GOGC` intercambia CPU por memoria**: de 25 a 400, el GC pasa de ~48 % a ~16 % de la CPU y los ciclos de ~100 a ~7, pero el pico de heap se multiplica por **20** (de ~100 MiB a ~2 GiB). El rendimiento mejora **2-4×**, con mucha dispersión.
- **El coste de GOGC alto es mayor del que dice la fórmula** cuando se asigna a ritmos extremos (aquí `58 MB × 5 ≈ 290 MB` esperados; se observaron 1.5-2.3 GiB) por la razón del "vivo" inflado explicada arriba. En un servicio con tasas de asignación normales el efecto es mucho menor.
- **`GOMEMLIMIT` como único mecanismo fue 4× más lento** que `GOGC=100` en esta carga extrema: mantener el heap pegado al límite obliga a recolectar con mucha frecuencia y a frenar a los que asignan. *Hipótesis* (no verificada): las asistencias de GC. Recomendación práctica: **mantén `GOGC` activo y usa `GOMEMLIMIT` como red de seguridad**, no como único control.
- **Green Tea vs clásico:** en esta carga las diferencias están **dentro del ruido** (rangos solapados, con una ligera ventaja de Green Tea en CPU del GC). No se puede concluir nada más con estas mediciones; sus beneficios se esperan con heaps grandes de muchos objetos pequeños.
- **El objetivo de optimización más rentable sigue siendo asignar menos** (ejercicio 1: 5× de mejora, sin tocar ninguna variable).

## Advertencia sobre estas mediciones

La carga es **sintética y extrema** (miles de millones de objetos/s, todos con punteros). La dispersión entre ejecuciones es grande (p. ej. 42-88 M obj/s con `GOGC=400`). Sirve para entender la **dirección y el orden de magnitud** de cada mando, no para elegir valores en producción: mide con tu carga real.

## Ejercicios propuestos

1. Reduce `-vivos` a 10 000 y a 1 000 000: ¿cómo cambia la fracción de CPU del GC?
2. Pasa `-workers 1` y compara: el GC "dedicado" del 25 % pesa distinto con un solo hilo.
3. Haz que los objetos no tengan punteros (`v [4]int64` sin `sig`) y observa cuánto baja el coste de marcar.
4. Añade `/gc/heap/live:bytes` y `/gc/limiter/last-enabled` (ejercicio 6) al programa para ver cuándo actúa el limitador de CPU.
