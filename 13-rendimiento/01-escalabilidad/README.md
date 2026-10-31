# Ejercicio 1: escalabilidad con el número de núcleos — medir, no suponer

## Teoría

"Más goroutines = más rápido" es una suposición, no una ley. Se mide con el **escalado fuerte**: la **misma cantidad de trabajo** repartida entre `k` workers.

| Magnitud | Fórmula | Ideal |
|--|--|--|
| **Speedup** | `S(k) = T(1) / T(k)` | `k` |
| **Eficiencia** | `E(k) = S(k) / k` | 100 % |

### Por qué casi nunca se llega al ideal

1. **Parte serial** (ley de Amdahl): `S(∞) = 1 / (1 − p)`. Un 5 % serial limita a 20×.
2. **Recursos compartidos** que se saturan: ancho de banda de memoria, caché, asignador de memoria/GC, locks, red, disco.
3. **Núcleos no equivalentes:** muchos portátiles y servidores modernos mezclan núcleos de *alto rendimiento* y de *eficiencia* (Apple Silicon, Intel híbridos). 8 núcleos ≠ 8 unidades de cómputo.
4. **Hyper-threading/SMT:** dos hilos lógicos comparten un núcleo físico (≈ 1.2-1.3× un solo hilo, no 2×).
5. **Sobrecoste de coordinación** (crear goroutines, sincronizar, repartir, fusionar).
6. **Frecuencia y temperatura:** con todos los núcleos activos la CPU baja de frecuencia.

### El tipo de carga decide

| Carga | Cuello de botella | Escala hasta… |
|--|--|--|
| **Cálculo puro** (registros/caché) | núcleos | ≈ nº de núcleos "equivalentes" |
| **Memoria** (recorrer datos grandes) | **ancho de banda de memoria** | unos pocos núcleos: el bus se satura |
| **Asignaciones** | asignador + GC | poco: ver módulo 12 |
| **Bloqueos / E/S** | el recurso externo | mucho más allá de `NumCPU` (módulo 1) |

### `GOMAXPROCS` y más workers que núcleos

Para trabajo de CPU, **más goroutines que `GOMAXPROCS` no ayudan** (solo añaden cambios de contexto). Para trabajo bloqueante sí.

## Qué hace el programa

Tres cargas de trabajo fijo, repartidas entre 1, 2, 4, 6, 8 y 16 workers (mejor de 3 mediciones):

1. **Cálculo puro:** 600 M iteraciones de un xorshift (sin tocar memoria).
2. **Memoria:** sumar un array de 512 MiB (4 pasadas, mucho mayor que cualquier caché).
3. **Asignaciones:** 24 M objetos pequeños con puntero.

Imprime tiempo, speedup, eficiencia y una **estimación de la fracción paralela** por Amdahl.

```bash
go run .
```

## Resultados de referencia (Apple M1: 4 núcleos de rendimiento + 4 de eficiencia)

**Cálculo puro** (3 ejecuciones; rangos):

```
workers    tiempo     speedup   eficiencia
      1   1.18-1.26 s   1.00x      100%
      2   600-627 ms    1.88-2.05x  ~97%
      4   326-343 ms    3.64-3.68x  ~92%     (una ejecución dio 2.74x: ruido del planificador del SO)
      6   254-269 ms    4.41-4.90x  ~78%
      8   221-239 ms    5.21-5.38x  ~66%
     16   229-232 ms    5.11-5.44x  ~33%     <- más workers que núcleos: nada que ganar
```

**Memoria:** `1 → 131 ms; 2 → 70 ms (1.9×); 4 → 45 ms (2.9×); 6/8/16 → 45-50 ms (2.6-2.9×)`: **se estanca a partir de 4 workers**.

**Asignaciones:** `2 → 1.6×; 4 → 2.4×; 8 → 2.5×; 16 → 2.7×`.

## Conclusiones

- **Cálculo puro: 5.3× con 8 workers, no 8×.** La causa no es una parte serial sino el hardware: 4 núcleos de rendimiento dan ~3.7× y los 4 de eficiencia aportan solo ~1.6× más (cada uno rinde en torno al 40 % de uno de rendimiento): `3.7 + 4 × 0.4 ≈ 5.3`. A **16 workers no hay mejora**.
- **Cuidado con la estimación de Amdahl:** el programa calcula "92-93 % paralelo", pero ese modelo supone núcleos iguales; aquí el 7 % "serial" es en realidad hardware heterogéneo. **Un modelo mal aplicado produce una conclusión equivocada** (que habría que "paralelizar más" cuando ya no hay nada serial).
- **Memoria: 2.9× como techo.** La carga no es de cálculo: el cuello de botella es el ancho de banda de memoria, que ya se satura con 4 hilos. Añadir núcleos no lo arregla; sí reducir el recorrido de memoria (compactar datos, procesar por bloques que quepan en caché, tipos más pequeños).
- **Asignaciones: 2.5-2.7×.** El asignador y el GC son recursos compartidos (módulo 12, ejercicio 1).
- **La misma máquina, tres techos distintos (5.3×, 2.9×, 2.7×):** no existe "el" speedup de un programa; depende de qué recurso lo limita.

## Ejercicios propuestos

1. Ejecuta con `GOMAXPROCS=4` y compara: ¿cuánto pierdes con los núcleos de eficiencia?
2. Cambia el array de memoria a 4 MiB (cabe en la caché L2): ¿cambia el techo?
3. Pasa a escalado **débil** (cada worker hace el mismo trabajo que el caso de 1 worker) y mide `T(k)/T(1)`.
4. Mide en otra máquina (o un contenedor con `--cpus`) y compara los techos.
