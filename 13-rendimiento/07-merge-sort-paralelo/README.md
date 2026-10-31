# Ejercicio 7: ordenación por mezcla en paralelo

## Teoría

Merge sort es el ejemplo clásico de **divide y vencerás** paralelo: las dos mitades son independientes, así que se ordenan **a la vez**; luego se mezclan.

```
              [ 8 M elementos ]
             /                 \
   [4 M] (goroutine)        [4 M] (esta goroutine)     <- se ordenan EN PARALELO
        …                         …
             \                 /
              mezclar(2 mitades)                       <- paso secuencial: n operaciones
```

### Tres decisiones de diseño

1. **Umbral (*cutoff*) de paralelización.** Crear una goroutine por cada división hasta tamaño 1 produce millones de goroutines: el coste de crearlas y planificarlas supera al trabajo. Por debajo de un umbral se recurre al algoritmo **secuencial**. Un umbral demasiado **alto** deja núcleos sin usar.
2. **Rangos disjuntos, sin sincronización de datos.** Cada llamada recursiva trabaja sobre `a[lo:hi]` y `tmp[lo:hi]` **distintos**: no hay carreras, solo un `WaitGroup` para esperar a las dos mitades.
3. **La mezcla es el cuello de botella (Amdahl).** Con mezcla secuencial, el último paso procesa los n elementos en un único hilo. Se puede paralelizar con la **mezcla por mediana**: tomar la mediana `x[mx]` del array mayor, buscar su posición `my` en el otro con búsqueda binaria, y mezclar en paralelo `(x[:mx], y[:my])` y `(x[mx+1:], y[my:])`, que son independientes. La **copia** final también se divide.

### Otros detalles

- Merge sort necesita un búfer auxiliar de tamaño n (memoria extra); `slices.Sort` (pdqsort) ordena *in situ*.
- **Insertion sort para trozos pequeños** (≤ 24) reduce la recursión.
- Un orden **estable** no es necesario para enteros; para estructuras sí, y la mezcla por mediana pide cuidado con los empates.

## Qué contiene

- `ordenar.go`: `OrdenarSecuencial`, `OrdenarParalelo` con `Config{Umbral, MezclaPar, UmbralMezcla, Goroutines}`, `mezclarParalelo`, `copiarParalelo`.
- `main.go`: 8 M enteros aleatorios; mide el efecto del umbral y de la mezcla paralela, y cuenta las goroutines.
- `ordenar_test.go`: todas las variantes ordenan bien nueve tipos de entrada (vacío, uno, dos, iguales, ascendente, descendente, muchos duplicados, aleatorios) frente a `slices.Sort` con `-race`; y el umbral limita las goroutines.

```bash
go run .
go test -race .
go test -bench Ordenar -run xxx
```

## Resultados de referencia (8 M enteros, mejor de 3, M1)

```
slices.Sort (pdqsort)                                      615 ms
merge sort secuencial (referencia)                         670 ms    1.00x

-- el umbral --
paralelo, mezcla secuencial, umbral=32                     262 ms    2.56x   goroutines: 262 143
paralelo, mezcla secuencial, umbral=1000                   181 ms    3.70x   goroutines:   8 191
paralelo, mezcla secuencial, umbral=16000                  178 ms    3.77x   goroutines:     511
paralelo, mezcla secuencial, umbral=256000                 182 ms    3.68x   goroutines:      31
paralelo, mezcla secuencial, umbral=1000000                185 ms    3.62x   goroutines:       7
paralelo, mezcla secuencial, umbral=4000000                362 ms    1.85x   goroutines:       1

-- paralelizando también la mezcla --
paralelo + mezcla paralela, umbral=16000                   177 ms    3.79x   goroutines: 8 711
paralelo + mezcla paralela, umbral=256000                  146 ms    4.58x   goroutines: 5 093
```

## Conclusiones

- **Hay una meseta amplia:** del umbral 1 000 al 1 000 000 el rendimiento es casi idéntico (3.6-3.8×) aunque el número de goroutines varía de 8 191 a **7**. Con trabajo de CPU, **unas pocas goroutines bien repartidas bastan**; no hacen falta miles.
- **Dos extremos malos:** con umbral **32** se crean **262 143 goroutines** y se pierde un 30 % (2.56× frente a 3.7×): el coste de crearlas y la presión sobre el planificador y el GC. Con umbral **4 M** solo hay **una** división (1 goroutine extra) y apenas 1.85×: faltan núcleos trabajando.
- **Amdahl en acción:** con mezcla secuencial el techo es 3.8×; paralelizar la mezcla y la copia lo sube a **4.58×** (146 ms). No llega a 8× porque (1) la ordenación mueve mucha memoria (el ancho de banda se satura, ejercicio 1) y (2) los 4 núcleos de eficiencia del M1 rinden menos.
- **La biblioteca estándar es muy buena:** `slices.Sort` (615 ms) supera a mi merge sort secuencial (670 ms). Mi versión paralela (146 ms) es **4.2× más rápida que `slices.Sort`**, a costa de n enteros de memoria extra y de usar todos los núcleos. Sigue valiendo la pena preguntarse si ese ahorro de tiempo justifica la complejidad; para n pequeño, `slices.Sort` gana.
- **Medir antes de paralelizar:** para tamaños por debajo de ~100 000 elementos, el coste de coordinar supera el beneficio.

## Ejercicios propuestos

1. Compara con una **quicksort paralela** (partición + dos mitades).
2. Haz que el umbral sea proporcional a `n / (k × NumCPU)` y evita configurarlo a mano.
3. Genéricos: ordenar `[]T` con una función de comparación (`cmp.Compare`) y compara el coste con `int`.
4. Mide el efecto de tamaños 10⁴, 10⁵, 10⁶, 10⁷ y encuentra el punto a partir del cual compensa paralelizar.
