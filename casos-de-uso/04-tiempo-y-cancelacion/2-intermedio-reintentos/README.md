# Caso 4.2 — Reintentos con backoff y jitter  ·  `intermedio`

> **Problema:** Una API falla a veces (503, red). Reintentar ayuda, pero reintentar al instante sobrecarga al servicio, reintentar un 400 es inútil y dormir con `time.Sleep` impide cancelar.

## Qué features usa y por qué

| Feature de Go | Papel en este caso |
|--|--|
| Backoff exponencial | la espera se duplica en cada intento, con tope |
| Jitter aleatorio | los clientes no reintentan a la vez (evita la «estampida») |
| `errors.Is(err, ErrTemporal)` | solo se reintenta lo que merece la pena; lo permanente falla ya |
| `time.Timer` + `select` con `ctx.Done()` | la espera es cancelable |
| `testing/synctest` | los tests verifican los instantes exactos con reloj virtual |

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ Fallos **transitorios** (red, 503, bloqueos de BD) y operaciones **idempotentes** (reintentar no duplica efectos).
- ✅ Siempre con un límite de intentos o un plazo.
- ❌ Operaciones no idempotentes (cobrar un pago) sin clave de idempotencia: reintentar puede duplicar.
- ❌ Errores de validación o 4xx: nunca se arreglan reintentando.

## Cómo ejecutarlo

```bash
go run .
go test -race -v .
```

## Qué observar (salida real, Apple M1, Go 1.26.5)

```
servicio inestable: err=<nil> tras 4 llamadas y 250ms (esperas ≈ 25-50, 50-100, 100-200 ms)      ← varía por el jitter
error permanente:   error permanente en el intento 1: 400: petición inválida (llamadas: 1, no se reintenta)
con plazo de 120 ms: cancelado tras 2 intento(s): context deadline exceeded
```

## Errores típicos

- Reintentar **sin backoff**: convierte un fallo parcial en una caída total.
- Backoff **sin jitter**: todos los clientes reintentan en el mismo instante.
- `time.Sleep` entre intentos: no se puede cancelar.
- Reintentar sin tope de tiempo o de intentos.
- Reintentar en cada capa de la pila: el factor se multiplica (3 capas × 3 intentos = 27).

## Profundiza en el curso

- [jitter y livelock](../../../15-diseno-avanzado/01-interbloqueos)
- [clasificar errores](../../../10-errores/05-errores-en-pipelines)
- [`Cause` para saber por qué se canceló](../../../05-context/04-cause-afterfunc-withoutcancel)
- [probar reintentos con tiempo virtual](../../../11-testing/04-synctest-aplicado)
- [backoff en el planificador de trabajos](../../../proyecto-final)

## Prueba tú

1. Añade un `Retry-After` devuelto por el servidor que prevalezca sobre el backoff.
2. Cuenta el tiempo total máximo posible para `MaxIntentos=5, Base=50ms, Tope=1s`.
3. Haz que `Reintentar` devuelva todos los errores (`errors.Join`).
