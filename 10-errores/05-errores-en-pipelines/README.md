# Ejercicio 5: errores en pipelines

## Teoría

En el módulo 8 los errores viajaban **como datos** (`Par[T]{Val, Err}`): el pipeline sigue y la etapa final decide. Eso conviene cuando cada elemento es independiente. Aquí se muestra la alternativa: **fallar rápido**, cuando un elemento inválido invalida todo el resultado.

```
generador ──► [lineas] ──► N parseadores ──► [nums] ──► sumador
   └────────────────── todas las etapas dentro de UN errgroup ──────────────────┘
```

### Diseño

- Cada etapa es una tarea del `errgroup` con el **contexto derivado**. El primer error lo cancela y las demás etapas salen por sus `select` con `ctx.Done()`.
- **Dueños del cierre** (módulo 3):
  - el generador cierra `chLineas` (`defer close`);
  - hay **varios** parseadores emitiendo en `chNums`, así que nadie los cierra individualmente: un **sub-grupo** `gp` agrupa a los parseadores y quien lo espera (`gp.Wait()`) cierra `chNums` cuando todos terminaron.
- El error de un parseador (`línea "5000x": ...`) sube por `gp.Wait()` a la tarea de la etapa 2, que lo devuelve al grupo principal; este cancela el contexto y las etapas 1 y 3 salen.
- `SumarLineas` devuelve el primer error y `0` como total (no hay resultado parcial fiable).

### ¿Datos o fallar rápido?

| | Error como dato | Fallar rápido (`errgroup`) |
|--|--|--|
| Un elemento malo | se reporta y se sigue | aborta todo |
| Resultado | parcial + lista de errores | todo o nada |
| Complejidad | baja | cierres y sub-grupos |
| Ejemplo | importar 10 000 filas e informar de las erróneas | validar un lote que debe ser atómico |

## Qué hace el programa

1. Entrada válida de 10 000 líneas.
2. Una línea inválida en la posición 5000.
3. Cancelación externa durante el proceso.
4. Cuenta las goroutines fugadas.

```bash
go run .
go test -race -timeout 60s .
```

## Resultado de referencia

```
1) válida:   total=49995000 (esperado 49995000) err=<nil> procesadas=10000
2) inválida: total=0 err=línea "5000x": strconv.Atoi: parsing "5000x": invalid syntax
   procesadas 5111 de 10000 líneas antes de abortar (el resto se evitó)
3) cancelación externa tras ~100 líneas: err=context canceled, procesadas=103 de 10000

goroutines fugadas: 0
```

## Conclusiones

- El pipeline se detuvo tras 5111 líneas (algo más de la 5000 porque los 4 parseadores trabajaban en paralelo y hubo líneas en vuelo).
- Con la cancelación externa se evitaron 9897 líneas.
- Ninguna goroutine quedó viva en ningún caso.

## Tests

Suma correcta con 1, 3 y 16 workers; un error detiene el pipeline (procesa menos de la mitad de 100 000 líneas); 20 ejecuciones con error sin fugas; contexto ya cancelado.

## Ejercicios propuestos

1. Recoge **todos** los errores de parseo con `errors.Join` en lugar de abortar.
2. Añade una etapa intermedia de filtrado y comprueba que los cierres siguen siendo correctos.
3. Devuelve el resultado parcial junto con el error (¿qué garantías das?).
