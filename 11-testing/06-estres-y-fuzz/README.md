# Ejercicio 6: pruebas de estrés y fuzzing

## Teoría

Una cola FIFO acotada (`Anillo`, buffer circular con mutex) se prueba con **tres técnicas complementarias**, cada una pensada para un tipo de fallo distinto.

| Técnica | Encuentra | No encuentra |
|--|--|--|
| **Fuzzing contra un modelo** (secuencial) | bugs de **lógica** que dependen de la *secuencia* de operaciones (p. ej., tras dar la vuelta al buffer) | bugs de sincronización |
| **Estrés + invariantes** (concurrente, `-race`, `-cpu`, `-count`) | pérdida/duplicado de elementos, deadlocks, errores de sincronización | bugs de lógica muy específicos |
| **Detector de carreras** (`-race`) | accesos sin sincronizar, aunque el resultado "parezca" bien | races que no se ejecutan |

### Fuzzing contra un modelo

1. Define el **modelo**: la implementación más simple posible (un `slice`).
2. Interpreta cada byte de la entrada como una operación (`par` = `Push`, `impar` = `Pop`).
3. Tras cada paso compara **resultado y estado** de la implementación real con el modelo.
4. `go test -fuzz` genera y muta entradas guiado por la cobertura; al encontrar un fallo lo **minimiza** y lo guarda en `testdata/fuzz/…`, que `go test` reejecuta después como **test de regresión**.

Sin `-fuzz`, `go test` ejecuta solo las semillas (`f.Add`) y los casos de `testdata`: es seguro dejar los `Fuzz*` en la suite.

### Estrés con invariantes

Lo importante no es "que no falle", sino **qué se comprueba**:
- **Conservación**: lo enviado es exactamente lo recibido (cuenta y suma de valores).
- **Orden FIFO**: solo se puede afirmar con *un* productor y *un* consumidor (con varios, el orden global entre consumidores no está definido).
- **Estado final**: `Len() == 0`.
- Ejecutar con `-race -cpu 1,2,8 -count=N`.

### Lección de `-cpu 1`: la espera activa

Los bucles de reintento (`for !q.Push(v) {}`) giraban sin ceder el turno. Con `GOMAXPROCS=1`, cada goroutine en bucle consumía una franja completa de ~10 ms (hasta que la preemption asíncrona la expropia) antes de que otra avanzara: el test pasó de **1.4 s a más de 60 s** (excedió el `-timeout`). Con `runtime.Gosched()` en el reintento (cesión cooperativa) volvió a **1.4 s**. Es un fallo de **rendimiento bajo pocas CPUs** que solo `-cpu 1` destapó. En código de producción: nunca espera activa sin ceder; mejor un `Cond` o un channel.

## Qué contiene

- `anillo.go`: `Anillo` (correcto).
- `anillo_test.go`: modelo, `FuzzAnillo`, `TestEstresConservacion`, `TestEstresOrdenFIFOUnoAUno`, `TestLenConcurrente`.
- `bugs_test.go` (`-tags bugs`): `anilloMalo` (bug al dar la vuelta), `FuzzAnilloMalo`, y `anilloRacy` (`Len` sin lock).

```bash
go test -race -cpu 1,2,8 -count=5 -timeout 120s .                       # estrés
go test -fuzz FuzzAnillo -fuzztime 5s -run xxx .                        # fuzz del bueno
go test -tags bugs -fuzz FuzzAnilloMalo -fuzztime 30s -run xxx .        # encuentra el bug
go test -tags bugs -race -run TestLenSinLock .                          # carrera
```

## Resultados de referencia

**Fuzz del anillo correcto (5 s):** 472 033 ejecuciones, sin fallos.

**Fuzz del anillo con bug** (encontrado en **0.2 s**):
```
bugs_test.go:45: op 9 Pop() = (48,true), modelo (50,true)
Failing input written to testdata/fuzz/FuzzAnilloMalo/171c8cfb9ab368e3

go test fuzz v1
[]byte("0101001211")            <- minimizada a 10 operaciones
```
La secuencia: `push, pop, push, pop, push, push, pop, push, pop, pop` con capacidad 4: tras dar la vuelta al buffer, `head` se reinicia a 1 en lugar de 0 y se devuelve el elemento equivocado. El test escrito a mano con 3 elementos **no lo habría visto**.

**Reejecución sin `-fuzz`:** el fichero guardado se ejecuta como regresión y vuelve a fallar hasta que se arregle el bug.

**Estrés (todas las CPUs):**
```
-cpu 1: ok  1.4s   -cpu 2: ok  1.2s   -cpu 8: ok  1.2s
-race -cpu 1,2,8 -count=5: ok 2.0s
```

**`Len` sin lock:**
```
test normal (5 repeticiones): ok            <- el valor "parece" razonable
con -race: WARNING: DATA RACE ... anilloRacy.Len()   <- el detector sí lo ve
```

## Conclusiones

- Un fuzzer con modelo encuentra en décimas de segundo un bug de lógica que exige una secuencia concreta y larga.
- El estrés por sí solo no detecta el `Len` sin lock; `-race` sí. Y `-race` sin estrés no ejercita suficiente concurrencia. Se combinan.
- `-cpu 1,2,8` no es decorativo: destapó un problema de espera activa.
- Guardar las entradas del fuzzer en `testdata/` convierte cada bug hallado en un test de regresión permanente.

## Ejercicios propuestos

1. Convierte la cola en bloqueante (`Push` espera si está llena) con `sync.Cond` y adapta el estrés (¿desaparece el `Gosched`?).
2. Añade al modelo una operación `Len` explícita como byte de entrada.
3. Escribe un test que verifique linealizabilidad básica: registra inicio/fin de cada operación y comprueba que existe un orden secuencial válido.
4. Ejecuta el fuzzer 10 minutos sobre `Anillo` con capacidades variables (que la capacidad también sea parte de la entrada).
