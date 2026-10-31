# Ejercicio 6: paralelismo de datos — map-reduce

## Teoría

**Paralelismo de datos**: la misma operación sobre partes distintas de los datos. El patrón general:

```
datos ──particionar──►  [parte 1] [parte 2] … [parte W]
                           │         │           │        (MAP: cada worker, a su ritmo, sin compartir)
                        resultado resultado   resultado
                           └─────────┴─────┬─────┘
                                       combinar            (REDUCE)
```

### Claves del diseño

1. **Particionar en trozos contiguos** (`datos[i*n/W : (i+1)*n/W]`): cada worker recorre memoria secuencial (buena para la caché y sin solapes).
2. **Estado local por worker**: cada uno agrega en **su propio** mapa/acumulador, **sin locks**.
3. **Combinar al final**. Aquí está el coste: si hay muchas claves, la fusión puede dominar.
4. Elegir bien el **número de partes** (≈ nº de núcleos para CPU; más si hay desigualdad entre partes).

### Las tres estrategias del ejercicio

| Estrategia | Cómo | Riesgo |
|--|--|--|
| **Mapa compartido + mutex** | todos escriben en un mapa global | el lock se convierte en un cuello de botella: puede ser **más lento que el secuencial** |
| **Locales + fusión secuencial** | cada worker su mapa; una goroutine fusiona | la fusión es la parte serial (Amdahl) |
| **Locales + fusión paralela** | R "reductores"; el reductor r es dueño de las claves con `hash%R == r`, recorre todos los mapas locales y se queda con las suyas | los reductores escriben en mapas distintos ⇒ sin locks; el resultado queda **particionado** |

### Cuándo un reduce paralelo ayuda

Cuando el número de claves distintas es **grande** respecto a los datos: la fusión es entonces una parte significativa del trabajo.

## Qué contiene

- `mapreduce.go`: `Secuencial`, `CompartidoConMutex`, `LocalYMergeSecuencial`, `LocalYMergeParalelo` (genéricas en entrada, clave y valor), más `trozos` y `Aplanar`.
- `main.go`: 20 M elementos con una distribución sesgada; dos escenarios (1 000 claves distintas y 2 000 000).
- `mapreduce_test.go`: todas las estrategias coinciden con la secuencial (`-race`), las particiones son disjuntas, la suma de recuentos se conserva, entrada vacía y pequeña, y los trozos cubren todo.

```bash
go run .
go test -race .
```

## Resultados de referencia (M1, 8 núcleos; mejor de 3)

**Pocas claves (1 000):** secuencial 245 ms.

```
workers   mapa compartido+mutex   locales+fusión secuencial   locales+fusión paralela
      2     1.509 s  (0.16×)           130 ms (1.88×)             126 ms (1.94×)
      4     2.633 s  (0.09×)            73 ms (3.37×)              70 ms (3.52×)
      8     3.303 s  (0.07×)            69 ms (3.53×)              60 ms (4.06×)
```

**Muchas claves (2 000 000):** secuencial 1.67 s.

```
workers   mapa compartido+mutex   locales+fusión secuencial   locales+fusión paralela
      2     4.014 s  (0.42×)           1.461 s (1.14×)            1.417 s (1.18×)
      4     5.399 s  (0.31×)           1.481 s (1.13×)            1.116 s (1.50×)
      8     5.811 s  (0.29×)           1.405 s (1.19×)            1.287 s (1.30×)
```

## Conclusiones

- **El mapa compartido con mutex es desastroso:** con 8 workers es **13× más lento que el código secuencial** (3.3 s frente a 245 ms) y empeora al añadir workers (1.5 → 2.6 → 3.3 s). La contención del lock convierte "paralelizar" en "ralentizar".
- **Con estado local el escalado es bueno si hay pocas claves:** 3.5-4.1× con 8 workers (techo por la heterogeneidad del chip, ejercicio 1).
- **La fusión paralela ayuda poco con pocas claves** (60 vs 69 ms) porque la fusión es insignificante.
- **Con muchas claves, el speedup se hunde a 1.2-1.5×.** Tres causas: (1) cada worker construye un mapa casi tan grande como el global (las claves se repiten entre trozos), lo que **multiplica la memoria** y el trabajo total; (2) insertar en mapas enormes está limitado por **fallos de caché** (acceso aleatorio a memoria), igual que el techo de 2.9× del ejercicio 1; (3) la fusión suma trabajo que el secuencial no tiene. La fusión paralela mejora algo (1.5× frente a 1.1×), pero no cambia el panorama.
- **Lección:** "paralelizar" solo da beneficio si el trabajo adicional que introduce (duplicar estado, fusionar) es pequeño frente al ahorro. Para espacios de claves enormes, una alternativa es **particionar por clave antes de agregar** (cada worker solo procesa sus claves), a costa de leer los datos varias veces o de un *shuffle*; no se probó aquí.

## Un bug que encontró un test

`LocalYMergeSecuencial` accedía a `locales[0]` y entraba en **pánico con una entrada vacía** (no se crea ningún trozo). Lo destapó `TestEntradaVaciaYPequena`, que se había escrito precisamente para los bordes. Se corrigió devolviendo un mapa vacío.

## Ejercicios propuestos

1. Implementa el reparto por hash **antes** de agregar y mide con 2 000 000 de claves.
2. Cambia las claves a `string` y observa el efecto del coste de hashing y de memoria.
3. Usa un **pool de workers con trozos pequeños** (más trozos que workers) y compara con trozos grandes: ¿ayuda con datos desiguales?
4. Haz que `Aplanar` sea paralelo; ¿compensa?
