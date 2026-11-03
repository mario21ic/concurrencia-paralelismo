# Caso 8.3 — Memoria y caché: lo que limita al paralelismo sin que lo veas  ·  `avanzado`

> **Problema:** El mismo reparto en `k` workers da aceleraciones muy distintas según *qué recurso* limita al trabajo. ¿Cómo reconozco el cuello de botella antes de añadir más núcleos?

## Qué features usa y por qué

| Kernel | Qué hace | Qué lo limita |
|--|--|--|
| **A** · acumulador local | CPU pura, la variable vive en un registro | solo los núcleos disponibles |
| **B1** · acumuladores contiguos | CPU pura, pero cada worker escribe `acum[w]` en **cada iteración** y los 8 acumuladores caen en la misma línea de caché | **false sharing**: la línea «rebota» entre núcleos |
| **B2** · acumuladores a 128 B | igual que B1, con cada acumulador en su propia línea | los núcleos (y el coste de ir a memoria en cada iteración) |
| **C** · suma de 256 MB | recorre un slice enorme, casi sin cálculo | el **ancho de banda de memoria**, que es compartido |

Features de Go: `sync.WaitGroup.Go`, slices de acumuladores con separación (`salto = 16` palabras = 128 B, la línea de caché del M1; en x86 son 64 B), y una medición **intercalada** (mejor de 5, rotando los kernels) para que la carga ajena de la máquina afecte a todos por igual.

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ Cuando un paralelismo «no escala» y no sabes por qué: ejecuta kernels sencillos que aíslen cada hipótesis (CPU, caché, memoria).
- ✅ Regla práctica: **acumula en variables locales** y escribe el resultado una vez al final; si hay que escribir en un array compartido, separa los datos de cada worker por líneas de caché.
- ❌ No «rellenes» (padding) todo por si acaso: aumenta la memoria. Hazlo solo donde el perfil o la medición lo justifiquen.
- ❌ Para un trabajo limitado por memoria, más núcleos no ayudan; lo que ayuda es tocar menos bytes (filtrar antes, estructuras compactas, procesar por bloques que quepan en caché).

## Cómo ejecutarlo

```bash
go run .
go test -race -v .
```

## Qué observar (salida real, Apple M1, Go 1.26.5)

```
GOMAXPROCS=8. Aceleración frente a 1 worker (mejor de 5, kernels intercalados).

                                               k=1            k=2            k=4            k=8           
A  cálculo, acumulador local                   356ms          181ms (2.0×)   96ms (3.7×)    77ms (4.6×)   
B1 acumuladores contiguos (false sharing)      1.756s         966ms (1.8×)   490ms (3.6×)   484ms (3.6×)  
B2 acumuladores a 128 B (sin false sharing)    1.77s          908ms (1.9×)   467ms (3.8×)   330ms (5.4×)  
C  suma de 256 MB (ancho de banda)             14ms           7.2ms (1.9×)   4.9ms (2.9×)   5.8ms (2.4×)  
```

(Hubo carga ajena —una videollamada— durante la medición; en las dos ejecuciones en que se comparó con este mismo método, las cifras de B1/B2 se repitieron con diferencias de pocos puntos.)

Cómo leerlo:

- **A** escala casi lineal hasta 4 y llega a 4.6× con 8 (núcleos de rendimiento + eficiencia, como en [8.1](../1-basico-speedup)).
- **B**: B1 y B2 hacen *exactamente el mismo cálculo*, pero con 8 workers B1 tarda **484 ms y B2 330 ms (1.5× más lento)** solo por la posición de los acumuladores en memoria. Con 2 y 4 workers casi no se nota (1.8× frente a 1.9×, 3.6× frente a 3.8×) y con 8 sí: el problema **aparece al subir `k`**, por eso hay que probar con el máximo de núcleos. (El efecto es más moderado que en [13/02](../../../13-rendimiento/02-false-sharing) porque aquí cada iteración ya es lenta: B2 con 1 worker tarda 1.77 s frente a 356 ms de A, ya que hay que ir a memoria en cada vuelta.)
- **B2 con 1 worker es 5× más lento que A**: escribir en memoria en cada iteración en vez de usar un registro cuesta más que el reparto. La optimización más barata sigue siendo el acumulador local.
- **C** sube a 2.9× con 4 workers y **baja a 2.4× con 8**: sumar 256 MB casi no calcula; el límite son los bytes por segundo que entrega la memoria (≈ 19 GB/s con un worker, ≈ 55 GB/s en el mejor caso: 268 MB en 14 ms y en 4.9 ms), compartidos por todos los núcleos.

## Errores típicos

- Pensar que «no comparto datos» garantiza que no hay contención: **compartir la línea de caché** basta.
- Paralelizar un recorrido de memoria y esperar escalado lineal.
- Medir solo con `k` pequeño y descubrir el false sharing en producción con 64 núcleos.
- Rellenar con una constante de 64 B en un procesador de líneas de 128 B (M1/M2): usa `cpu.CacheLinePad` de `golang.org/x/sys/cpu`.

## Profundiza en el curso

- [false sharing, con la tabla completa de separaciones](../../../13-rendimiento/02-false-sharing)
- [contador fragmentado](../../../13-rendimiento/03-contador-fragmentado) y su caso de uso: [2.3](../../02-estado-compartido/3-avanzado-contadores)
- [escalabilidad y sus techos](../../../13-rendimiento/01-escalabilidad)
- [perfiles con pprof](../../../12-profiling/01-pprof-cpu)

## Prueba tú

1. Cambia `salto` a 8 (64 B) y vuelve a medir B2: en este chip, ¿basta? (la tabla de [13/02](../../../13-rendimiento/02-false-sharing) dice que no).
2. Añade un kernel D: igual que C pero con un slice de 512 KB repetido muchas veces (cabe en caché). ¿Escala mejor?
3. Cambia C para que cada worker recorra el slice con saltos de 16 elementos: ¿qué le pasa a la velocidad con un solo worker?
