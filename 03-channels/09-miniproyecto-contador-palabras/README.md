# Mini-proyecto 1: contador de palabras concurrente

## Enunciado

Contar la frecuencia de palabras en varios archivos de texto en paralelo y mostrar las más frecuentes.

```bash
go run . -dir textos -top 10 -workers 4 -min 4
go test -race -count=20 .
```

| Flag | Descripción | Defecto |
|--|--|--|
| `-dir` | directorio con `*.txt` | `textos` |
| `-top` | cuántas palabras mostrar | 10 |
| `-workers` | goroutines que cuentan | 4 |
| `-min` | longitud mínima de palabra | 4 |

## Diseño

```
archivos ──► [jobs] ──► worker 1 ─┐
                        worker 2 ─┼─► [resultados] ──► agregador (main) ──► top-N
                        worker N ─┘
```

Decisiones:

1. **Cada worker cuenta en un map local.** Sin locks, sin races: nadie más lo ve.
2. **Los maps viajan por un channel** (se transfiere la propiedad). Tras enviarlo, el worker no lo toca.
3. **Un único agregador** es dueño del map global y lo fusiona. No hay memoria compartida.
4. **Cierres con dueño claro:**
   - el productor de `jobs` cierra `jobs` (`defer close(jobs)`);
   - un coordinador cierra `resultados` tras `wg.Wait()`.
5. **Los errores viajan en el resultado** (`resultado.err`); un archivo ilegible no bloquea a los demás.
6. **Desempate estable** al ordenar el top (por frecuencia y luego alfabético) para que la salida sea determinista y testeable.
7. **Versión secuencial de referencia** para verificar que la concurrente da el mismo resultado.

## Resultado de referencia

```
4 archivos, 91 palabras distintas
 1. goroutines       6
 2. channel          5
 3. concurrencia     4
 4. paralelismo      4
 5. buffer           3
 6. cola             3
 7. para             3
 8. diseñado         2
```

Tests (`main_test.go`, pasan con `-race -count=20`):
- concurrente == secuencial para 1, 2 y 8 workers;
- un archivo inexistente produce un error sin bloquear ni perder los demás datos;
- el orden del top es determinista.

## Conceptos del módulo que se aplican

- channels sin buffer para trabajos y resultados
- direccionalidad en las firmas y propiedad del cierre
- `range` sobre channels cerrados
- ausencia de deadlocks por cierres correctos
- `WaitGroup.Go` + coordinador que cierra `resultados`

## Extensiones propuestas

1. **Medir:** genera 200 archivos grandes y mide tiempo con `-workers` 1, 2, 4, 8, 16. ¿Dónde se estanca? (módulo 1: CPU vs I/O-bound, Amdahl). ¿Qué parte es serial?
2. **Reducir el cuello del agregador:** haz que cada worker acumule en un map propio a lo largo de varios archivos y envíe uno solo al final.
3. **Recorrido recursivo** de directorios con `filepath.WalkDir`, alimentando `jobs` desde ahí.
4. **Cancelación:** detener todo con `Ctrl+C` (adelanto del módulo 5, `context`).
5. **Leer de stdin** y repartir por líneas en bloques a los workers.
