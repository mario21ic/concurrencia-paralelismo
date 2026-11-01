# Caso 4.1 — Poner un plazo  ·  `básico`

> **Problema:** Una llamada puede tardar segundos (o colgarse), pero el usuario no espera más de 200 ms. Hay tres formas de limitarla; solo una es del todo correcta.

## Qué features usa y por qué

| Feature de Go | Papel en este caso |
|--|--|
| `select` + `time.After` | forma básica, pero con un channel **sin buffer** deja una goroutine bloqueada para siempre |
| channel con buffer 1 | la goroutine envía y termina aunque nadie la espere (no fuga; la operación sigue ejecutándose) |
| `context.WithTimeout` | el contexto **llega a la operación**, que se cancela de verdad y libera recursos |
| `time.NewTimer` + `Stop` | evita dejar temporizadores vivos si la operación termina antes |

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ Cualquier llamada que pueda tardar: red, BD, subprocesos.
- ✅ Prefiere la forma con `ctx` siempre que puedas modificar la operación (o ya acepte `ctx`).
- ❌ Operación que **no** acepta `ctx` (código ajeno, cgo): usa la forma con buffer 1 y asume que seguirá gastando recursos hasta terminar.
- ❌ Un plazo no sustituye a la validación de entrada ni a los límites de tamaño.

## Cómo ejecutarlo

```bash
go run .
go test -race -v .
```

## Qué observar (salida real, Apple M1, Go 1.26.5)

```
ConPlazoMal   ×100: goroutines que SOBREVIVEN = 100 (bloqueadas enviando)
ConPlazoSinCtx×100: goroutines que sobreviven = 0
ConPlazoCtx: error=plazo vencido tras 200ms (la operación se canceló, no quedó trabajando)
```

## Errores típicos

- `select` con `time.After` y un channel sin buffer: fuga de una goroutine **por llamada**.
- Crear `context.WithTimeout` y olvidar `defer cancel()`: el temporizador sigue vivo hasta que vence.
- Pasar `context.Background()` a una operación: nada la podrá cancelar.
- Un plazo por intento que suma mucho más que el plazo total: usa un contexto padre.

## Profundiza en el curso

- [timeouts con `select`](../../../04-select-timers/03-timeouts)
- [plazos con `context`](../../../05-context/02-timeout-deadline)
- [operaciones que cooperan](../../../05-context/03-cancelacion-cooperativa)
- [temporizadores desde Go 1.23](../../../04-select-timers/05-timers-go123)
- [detectar la fuga de `ConPlazoMal`](../../../11-testing/05-fugas)

## Prueba tú

1. Mide con `go test -race` y `goleak` cuántas goroutines deja `ConPlazoMal`.
2. Haz que `lentaCtx` devuelva un valor parcial al cancelarse.
3. Encadena dos operaciones bajo el MISMO plazo total.
