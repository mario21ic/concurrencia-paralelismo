# Ejercicio 4: `synctest` en componentes reales

## Teoría

Tres componentes con comportamiento temporal que en tiempo real serían **lentos de probar o imposibles de probar con exactitud**:

| Componente | Comportamiento temporal |
|--|--|
| `Reintentar` | espera `espera·2^i` entre intentos (backoff exponencial); respeta `ctx` |
| `Debouncer` | ejecuta la función solo tras `d` de silencio; cada nueva llamada reinicia la cuenta atrás |
| `CacheTTL` | las entradas caducan al cumplirse su TTL; un barredor las elimina periódicamente |

### Patrón de test

```go
synctest.Test(t, func(t *testing.T) {
    ini := time.Now()
    err := Reintentar(t.Context(), 4, 100*time.Millisecond, siempreFalla)
    if got := time.Since(ini); got != 700*time.Millisecond { ... }   // exacto
})
```

- **Aserciones exactas** sobre la duración (`== 700ms`), no rangos.
- `Sleep` en el test = "avanzar el reloj"; `synctest.Wait()` = "deja que todo lo pendiente ocurra".
- Para probar el **límite** de un TTL: dormir `ttl - 1ns` (sigue vivo) y luego `1ns` más (vencida).
- **Barredores y goroutines de fondo:** pasar `t.Context()` y dejar que se cancele al final; si el barredor no obedeciera al contexto, `Test` entraría en pánico (`TestBarredorTerminaConElContexto`).
- Los contadores que escriben callbacks diferidos (`time.AfterFunc`) usan `atomic` (lección del ejercicio 3).

## Qué prueban los tests

| Test | Resultado verificado |
|--|--|
| `ReintentarSeRecupera` | falla 2 veces, éxito a la 3.ª; espera total **exactamente 300 ms** (100 + 200) |
| `ReintentarAgotaIntentos` | 4 intentos ⇒ espera **700 ms** (100 + 200 + 400); el error envuelve `ErrTransitorio` |
| `ReintentarBackoffGrande` | 10 intentos con base de 1 minuto ⇒ **511 minutos (8 h 31 min)** de backoff, en microsegundos |
| `ReintentarCancelacion` | con plazo de 250 ms se detiene a los **250 ms exactos** con `DeadlineExceeded` |
| `DebounceAgrupaRafagas` | 5 llamadas separadas 50 ms ⇒ **un solo disparo**, 100 ms después de la última |
| `DebounceLlamadasEspaciadas` | llamadas separadas 150 ms ⇒ 3 disparos |
| `CacheExpiraExactamenteEnElTTL` | viva a 59 m 59.999999999 s; vencida a 1 h exacta |
| `BarredorEliminaSinGet` | 100 entradas caducadas se eliminan en el barrido de los 5 min, sin llamar a `Get` |
| `BarredorTerminaConElContexto` | el barredor sale al cancelar `t.Context()` |

```bash
go test -race -count=50 -timeout 120s .
go test -v .
```

## Resultado de referencia

```
--- PASS: TestReintentarSeRecupera (0.00s)
--- PASS: TestReintentarAgotaIntentos (0.00s)
--- PASS: TestReintentarBackoffGrande (0.00s)
--- PASS: TestReintentarCancelacion (0.00s)
--- PASS: TestDebounceAgrupaRafagas (0.00s)
--- PASS: TestDebounceLlamadasEspaciadas (0.00s)
--- PASS: TestCacheExpiraExactamenteEnElTTL (0.00s)
--- PASS: TestBarredorEliminaSinGet (0.00s)
--- PASS: TestBarredorTerminaConElContexto (0.00s)
ok   0.174s
```

## Conclusiones

- El conjunto simula **unas 9.7 horas** de tiempo (8.5 h solo del backoff grande, más ~1 h de TTL) y tarda **0.17 s** de reloj; 50 repeticiones con `-race` tardan 1.4 s en total.
- Las aserciones son exactas, así que un cambio de `<` a `<=` en el TTL, o un backoff mal calculado, **rompen** el test; con tiempo real y márgenes de tolerancia pasarían desapercibidos.
- El mismo código en tiempo real (`ReintentarBackoffGrande`) sería **imposible** de ejecutar en una suite.

## Ejercicios propuestos

1. Añade jitter al backoff (con una fuente aleatoria inyectada) y pruébalo con `synctest`.
2. Implementa un *circuit breaker* (abre tras N fallos, semiabierto tras T) y pruébalo.
3. Prueba el `Acumulador` del ejercicio 2 con un `time.Ticker` real dentro de la burbuja.
4. Reescribe el `Limiter` del módulo 8 con `time.Now()` y prueba la reposición de tokens.
