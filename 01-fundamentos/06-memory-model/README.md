# Ejercicio 6: memory model y happens-before

## Teoría

El **Go Memory Model** especifica cuándo una lectura de una variable en una goroutine **está garantizada** de ver la escritura hecha por otra. La respuesta: solo si existe una relación **happens-before** entre ambas.

Compilador y CPU pueden reordenar instrucciones y mantener valores en registros/cachés. Sin sincronización, otra goroutine puede ver valores viejos, parciales o en otro orden.

### Data race

Dos accesos concurrentes a la misma posición de memoria, al menos uno de escritura, sin happens-before entre ellos. Es **comportamiento indefinido en la práctica**: contadores incorrectos, bucles infinitos, corrupción de datos (sobre todo con slices, maps, strings, interfaces).

### Qué crea happens-before

| Operación | Garantía |
|--|--|
| `go f()` | la sentencia `go` ocurre antes de que `f` empiece |
| Envío en channel | ocurre antes de que la recepción correspondiente termine |
| Cierre de channel | ocurre antes de una recepción que devuelve por el cierre |
| Recepción en channel **sin buffer** | ocurre antes de que el envío termine |
| `mu.Unlock()` n | ocurre antes de `mu.Lock()` n+1 |
| `wg.Done()` | ocurre antes de que `wg.Wait()` retorne |
| `sync.Once` | el retorno de `f` ocurre antes de que cualquier `Do` retorne |
| Operaciones `sync/atomic` | son secuencialmente consistentes: un `Load` que ve un `Store` queda sincronizado con él |

Nota: `time.Sleep` **no** crea happens-before. Nunca "arregles" una race con un sleep.

## Qué hace el programa

Incrementa un contador desde 100 goroutines × 10 000 veces (esperado: 1 000 000) de varias formas:

```bash
go run . -modo=race              # incorrecto
go run . -modo=mutex             # correcto
go run . -modo=atomic            # correcto
go run . -modo=channel           # correcto
go run -race . -modo=race        # el detector reporta la race
go run . -modo=publicar-bien     # publicación segura con atomic
go run -race . -modo=publicar-mal   # race; sin -race podría colgarse
```

> ⚠️ `publicar-mal` sin `-race` puede no terminar jamás (el compilador puede hoistear la lectura de `listoMal`). Ejecútalo con `-race` o prepárate para `Ctrl+C`.

## Resultado de referencia

```
race:    242776 (esperado 1000000)    <- pierde ~75 % de los incrementos
mutex:   1000000 (esperado 1000000)
atomic:  1000000 (esperado 1000000)
channel: 1000000 (esperado 1000000)
```

Con `-race`:

```
WARNING: DATA RACE
Read at 0x00c000096038 by goroutine 8:
  main.race.func1() main.go:27
Previous write at 0x00c000096038 by goroutine 7:
  main.race.func1() main.go:27
```

## Conclusiones

- `n++` **no es atómico**: son tres pasos (leer, sumar, escribir) que se entrelazan.
- El resultado de una race cambia en cada ejecución: no se puede probar su ausencia con un test que "pasó".
- Soluciones: `Mutex` (secciones críticas), `atomic` (valores simples), channel (transferir/agregar resultados; la versión `channel` ni siquiera comparte variables: cada goroutine acumula en local y envía).
- **Usa `-race` siempre** en desarrollo y CI: `go test -race ./...`. Cuesta ~2–10x de CPU/memoria, por lo que no va en producción.

## Ejercicios propuestos

1. Quita el `Lock` solo en la lectura final de una variable y comprueba que `-race` lo detecta.
2. Haz una race sobre un `map` con escritura concurrente: Go lo detecta en runtime con `fatal error: concurrent map writes`.
3. Usa `time.Sleep` para "esperar" a la goroutine en vez de `WaitGroup`, y verifica que `-race` sigue quejándose.
