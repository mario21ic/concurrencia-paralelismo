# Caso 2.1 — Contador de visitas  ·  `básico`

> **Problema:** 1 000 goroutines incrementan el mismo contador. `visitas++` no es atómico (leer, sumar, escribir): las actualizaciones se pisan y se pierden.

## Qué features usa y por qué

| Feature de Go | Papel en este caso |
|--|--|
| `sync.Mutex` | la sección crítica la ejecuta una goroutine cada vez; sirve para cualquier operación o estructura |
| `sync/atomic` (`atomic.Int64`) | una instrucción atómica de CPU; solo para UN valor con operaciones simples; más barato |
| `go run -race` | el detector de carreras: señala el acceso sin proteger |

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ **Mutex**: el dato es una estructura o la operación implica varios pasos.
- ✅ **atomic**: un contador, una bandera, un puntero que se intercambia.
- ❌ Mutex para un simple contador cuando la contención es alta: un atomic es más rápido (y fragmentar, más aún: caso 2.3).
- ❌ Atomic para proteger **dos** variables que deben cambiar juntas: cada una es atómica, pero el par no.

## Cómo ejecutarlo

```bash
go run .
go test -race -v .
```

Ver la carrera de datos: `go run -race . -mal`

## Qué observar (salida real, Apple M1, Go 1.26.5)

```
esperado:   1000000
con mutex:  1000000
con atomic: 1000000
sin proteger: 438909  (se pierden 561091 actualizaciones)      ← con `-mal`; el valor varía en cada ejecución

(go run -race . -mal  →  WARNING: DATA RACE ...)
```

## Errores típicos

- Pensar que «`x++` es una sola instrucción». No lo es.
- Olvidar `Unlock` en una ruta de salida temprana → usa `defer mu.Unlock()`.
- Copiar un `Mutex` (`go vet` avisa): hay que pasar un puntero.
- Proteger la **escritura** pero leer sin cerrojo: también es carrera.
- Mutex anidados en orden distinto → interbloqueo (módulo 15, ejercicio 1).

## Profundiza en el curso

- [`Mutex` paso a paso](../../../06-sync/01-mutex)
- [contadores atómicos](../../../07-atomic/01-contadores)
- [el error de «comprobar y luego actuar»](../../../06-sync/07-check-then-act)
- [cómo arreglar una carrera real](../../../02-goroutines/07-arreglar-race)
- [por qué ocurre: el modelo de memoria](../../../01-fundamentos/06-memory-model)

## Prueba tú

1. Cambia el contador por un `map[string]int` y razona por qué `atomic` ya no sirve.
2. Quita `defer`/`Unlock` en una rama: observa el interbloqueo.
3. Mide cuántas veces más rápido es `atomic` que `mutex` con 8 goroutines (adelanto: caso 2.3).
