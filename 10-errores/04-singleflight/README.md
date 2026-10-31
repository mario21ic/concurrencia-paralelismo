# Ejercicio 4: `singleflight`

## Teoría

`singleflight.Group` ejecuta una función **una sola vez** por clave mientras haya llamadas **simultáneas**; las demás esperan y reciben el mismo resultado.

```go
v, err, compartido := g.Do(clave, func() (any, error) { return consultarBD(clave) })
```

- `compartido` es `true` si el resultado se entregó a más de un llamador.
- `DoChan` devuelve un channel (para poder esperar con `select` y un contexto propio).
- `Forget(clave)` hace que la siguiente llamada ejecute de nuevo, sin esperar a la que está en curso.

### Estampida de caché (*thundering herd*)

Una clave muy popular caduca en la caché; llegan 200 peticiones simultáneas; las 200 van a la base de datos a la vez. `singleflight` colapsa las 200 en **una**.

```
petición ─► caché ─► (fallo) ─► singleflight.Do ─► BD (una vez) ─► guarda en caché ─► todos reciben el valor
```

### Lo que NO es

- **No es una caché**: solo agrupa llamadas que coinciden en el tiempo. Si llega otra después de que terminó, ejecuta de nuevo.
- Los **errores se comparten**: un fallo puntual llega a todos los que esperaban.

### La trampa de los contextos

La función se ejecuta con los argumentos (y el `ctx`) del **primer** llamador. Si ese contexto vence, la ejecución falla y **todos** los que esperaban reciben `context deadline exceeded`, aunque tuvieran tiempo de sobra.

Solución:
1. Ejecutar con un contexto desacoplado: `context.WithoutCancel(ctx)` (opcionalmente con un plazo propio).
2. Usar `DoChan` y que **cada llamador espere con `select`** sobre su propio contexto: abandona *su* espera sin afectar a los demás.

### Cuándo usarlo

Trabajo **idempotente y costoso** repetido por muchos llamadores: consultas a BD, resolución DNS, llamadas a APIs, generación de miniaturas. No lo uses para operaciones con efectos (escrituras) donde cada llamada debe ejecutarse.

## Qué hace el programa

1. 100 peticiones sin `singleflight` frente a 100 con él.
2. Claves distintas, llamadas secuenciales (no es caché).
3. Estampida de caché con 200 peticiones.
4. Errores compartidos.
5. La trampa del contexto y su solución con `DoChan` + `WithoutCancel`.

```bash
go run .
go test -race -timeout 60s .
```

## Resultado de referencia

```
1) sin singleflight: consultas a la BD = 100
2) con singleflight: consultas a la BD = 1 (resultado compartido por 100 llamadores)
3) 90 llamadas a 3 claves distintas: consultas = 3
4) 3 llamadas secuenciales: consultas = 3 (no recuerda resultados)
5) estampida con caché + singleflight: 200 peticiones => consultas = 1
6) un fallo llegó a 10 de 10 llamadores (todos ven el mismo error)
7) trampa del contexto: primero=context deadline exceeded | segundo=context deadline exceeded
8) con DoChan + WithoutCancel: impaciente=context deadline exceeded | paciente=<nil> | consultas=1
```

## Conclusiones

- De 100 consultas a 1 (y de 200 a 1 en la estampida).
- El punto 7 es el bug: el segundo llamador, con tiempo de sobra, falló por culpa del plazo del primero.
- El punto 8 lo resuelve: el impaciente abandona, el paciente obtiene su resultado, y la BD recibe una sola consulta.

## Ejercicios propuestos

1. Añade `Forget` cuando la consulta falla, para que el siguiente llamador reintente en lugar de recibir el error en caché.
2. Cuenta cuántos llamadores compartieron cada ejecución con un histograma.
3. Combina `singleflight` con la caché TTL del módulo 7 (`GetOrCompute`).
