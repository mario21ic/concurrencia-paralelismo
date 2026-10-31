# Módulo 4. `select` y temporizadores

**Objetivo:** coordinar varias fuentes de eventos, poner límites de tiempo y evitar las fugas típicas de los timers.

## Ejercicios

| # | Carpeta | Tema |
|---|---------|------|
| 1 | [01-select-basico](01-select-basico) | Múltiples casos, elección aleatoria, prioridad |
| 2 | [02-no-bloqueante](02-no-bloqueante) | `default`: envío/recepción no bloqueantes |
| 3 | [03-timeouts](03-timeouts) | `time.After`, `time.NewTimer`, plazo global vs inactividad |
| 4 | [04-ticker](04-ticker) | `time.Ticker`, ticks perdidos, heartbeat |
| 5 | [05-timers-go123](05-timers-go123) | Cambios de timers en Go 1.23+ |
| 6 | [06-habilitar-casos](06-habilitar-casos) | Casos de `select` con channels `nil` |
| 7 | [07-servidor-mas-rapido](07-servidor-mas-rapido) | **Ejercicio:** consultar 3 servidores, ganar el más rápido |

```bash
cd 04-select-timers/01-select-basico && go run .
```

## Resumen teórico

### `select`

```go
select {
case v := <-a:      // recepción
case b <- x:        // envío
case v, ok := <-c:  // recepción con ok
default:            // opcional: si ninguno está listo
}
```

Reglas:

1. Evalúa **una vez**, en orden, las expresiones de channel y de los valores a enviar.
2. Si **ninguno** está listo y no hay `default`, **bloquea** hasta que alguno lo esté.
3. Si **varios** están listos, elige **uno al azar** (distribución uniforme). No hay prioridad ni orden: no dependas de él.
4. Con `default`, nunca bloquea. Úsalo para operaciones *try*; **nunca** en un bucle cerrado sin pausa (espera activa).
5. `select {}` bloquea para siempre. Un `select` sin casos listos con todos los channels `nil` también.
6. Un caso con channel `nil` jamás se elige: sirve para deshabilitar casos.
7. `break` dentro de un `select` solo sale del `select`; usa una etiqueta para salir del bucle.

### Temporizadores (`time`)

| Herramienta | Uso |
|--|--|
| `time.Sleep(d)` | pausar la goroutine actual (no cancelable) |
| `time.After(d)` | channel que recibe una vez tras `d`; cómodo para timeouts puntuales |
| `time.NewTimer(d)` | timer con `Stop` y `Reset`; para reutilizar o cancelar |
| `time.AfterFunc(d, f)` | ejecuta `f` en su propia goroutine tras `d` |
| `time.NewTicker(d)` | señal periódica; ¡`Stop` al terminar! |
| `time.Tick(d)` | ticker sin `Stop`; solo para programas de vida corta |

### Cambios de Go 1.23 (por `go.mod` ≥ 1.23)

- Los timers y tickers **sin referencia** se recolectan aunque no hayan expirado ni se hayan detenido.
- El channel del timer es **síncrono** (`cap=0`, `len=0`): tras `Stop` o `Reset` **no se recibe ningún valor viejo**. Ya no hace falta el ritual de drenar `t.C`.
- Esto cambia el comportamiento según la versión declarada en `go.mod` (`GODEBUG=asynctimerchan=1` restaura lo antiguo).

### Patrones clave

- **Timeout de una operación:** `select` con `case <-time.After(d)`.
- **Plazo global en un bucle:** crea **un** timer fuera del bucle (no `time.After` en cada vuelta, que reinicia el plazo).
- **Timeout por inactividad:** `t.Reset(d)` tras cada mensaje.
- **Un channel de resultado con buffer** evita dejar goroutines atascadas cuando el receptor se fue por timeout.
- Un timeout **no cancela** el trabajo; solo deja de esperarlo. Cancelarlo de verdad es tema del módulo 5 (`context`).

## Preguntas de repaso

1. Si dos casos de un `select` están listos, ¿cuál se ejecuta?
2. ¿Por qué `for { select { default: } }` es mala idea?
3. ¿Qué diferencia hay entre usar `time.After` dentro de un bucle y un solo `Timer` con `Reset`?
4. ¿Qué pasa si el consumidor de un `Ticker` es más lento que el periodo?
5. ¿Qué cambió en Go 1.23 respecto a `Stop` y `Reset`?
6. ¿Por qué el channel de resultados de "el más rápido" debe tener buffer?

## Lecturas

- [Go 1.23 release notes: Timer changes](https://go.dev/doc/go1.23#timer-changes)
- [Go Concurrency Patterns (Pike)](https://go.dev/talks/2012/concurrency.slide)
- Documentación de [`time`](https://pkg.go.dev/time#Timer) y la especificación de [`select`](https://go.dev/ref/spec#Select_statements)
