# Caso 1.1 — Varias esperas a la vez  ·  `básico`

> **Problema:** Una página necesita datos de 8 sitios. Pedirlos uno tras otro suma las esperas; pedirlos a la vez las solapa.

## Qué features usa y por qué

| Feature de Go | Papel en este caso |
|--|--|
| `go f()` | lanza la tarea en una goroutine (coste ≈ 2 KB, sin hilo del SO) |
| `sync.WaitGroup.Go` (Go 1.25) | lanza **y** cuenta en una línea; `Wait()` bloquea hasta que todas terminan |
| `res[i] = …` por índice | cada goroutine escribe en SU posición: no hay dato compartido que proteger |

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ Las tareas son **independientes** y **esperan** (red, disco, BD): el tiempo total baja a la espera más larga.
- ✅ Sabes de antemano cuántas son y no necesitas frenar ni cancelar.
- ❌ Si una falla y quieres abortar las demás → caso 1.2 (`errgroup`).
- ❌ Si son miles y cada una abre una conexión → necesitas limitar (caso 5.1).
- ❌ Si el trabajo es **CPU puro**, solapar no ayuda: hay que repartir entre núcleos (concepto 6).

## Cómo ejecutarlo

```bash
go run .
go test -race -v .
```

## Qué observar (salida real, Apple M1, Go 1.26.5)

```
secuencial: 810ms
paralelo:   100ms   (primer resultado: "contenido de a.com")
```

## Errores típicos

- **Olvidar `Wait()`**: `main` termina y mata las goroutines a medias.
- **Escribir en una variable compartida** (`total += x`) desde varias goroutines: carrera de datos. Escribe por índice o protege (concepto 2).
- Capturar la variable del bucle: desde Go 1.22 cada vuelta tiene su propia `i`, `u`; antes había que copiarla.
- Llamar `wg.Add` *dentro* de la goroutine: puede ejecutarse después de `Wait`. Con `wg.Go` el problema desaparece.

## Profundiza en el curso

- [`WaitGroup`: contar y esperar](../../../02-goroutines/03-waitgroup)
- [el mismo problema con URLs reales](../../../02-goroutines/06-urls-concurrentes)
- [por qué `i` ya se puede capturar](../../../02-goroutines/02-variables-de-bucle)
- [concurrencia ≠ paralelismo](../../../01-fundamentos/01-concurrencia-vs-paralelismo)

## Prueba tú

1. Cambia la latencia a 0 y a 1 µs: ¿sigue compensando lanzar goroutines?
2. Haz que `descargar` devuelva un error: ¿qué cambia en `Paralelo`? (pista: caso 1.2)
3. Ejecuta con `-race` y mueve `res[i] =` a `res = append(res, …)`: observa el detector.
