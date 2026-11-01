# Caso 5.3 — Limitar la tasa por cliente  ·  `avanzado`

> **Problema:** Una API pública acepta como mucho 2 peticiones/s por cliente, con ráfagas de hasta 5. Un cliente abusivo no debe afectar a los demás, y el rechazo debe indicar CUÁNDO volver a intentarlo.

## Qué features usa y por qué

| Feature de Go | Papel en este caso |
|--|--|
| `rate.Limiter` (token bucket) | fichas que se rellenan a `tasa`/s hasta una ráfaga máxima |
| `Reserve()` + `Delay()` + `Cancel()` | saber cuánto habría que esperar y **no gastar ficha** si se rechaza |
| `map` + `Mutex` | un limitador por cliente; el cerrojo solo protege el mapa |
| poda de ociosos | evita que el mapa crezca sin límite |
| `http.Handler` + `429` + `Retry-After` | el cliente educado espera en vez de insistir |

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ APIs públicas, protección frente a abuso, respetar cuotas de terceros (tú como cliente: `lim.Wait(ctx)`).
- ✅ Cuando quieres suavizar ráfagas y permitirlas hasta un tope.
- ❌ Quieres limitar **concurrencia**, no tasa → semáforo (5.1).
- ❌ Límites globales entre varios procesos → necesitas un almacén compartido (Redis); `rate` es por proceso.

## Cómo ejecutarlo

```bash
go run .
go test -race -v .
```

## Qué observar (salida real, Apple M1, Go 1.26.5)

```
cliente «abusivo» lanza 8 peticiones seguidas (ráfaga 5, 2/s):
  #1 → 200   #2 → 200   #3 → 200   #4 → 200   #5 → 200
  #6 → 429  Retry-After="1"   #7 → 429   #8 → 429
cliente «tranquilo» a la vez → 200 (su cuota es independiente)
«abusivo» tras esperar 0,6 s → 200
```

## Errores típicos

- Un único limitador global para todos los clientes: uno abusivo bloquea a todos.
- No podar el mapa: pérdida de memoria con clientes de un solo uso.
- Rechazar **gastando** la ficha: insistir empeora la espera (por eso `Cancel`).
- Confundir `Allow` (no espera) con `Wait` (bloquea hasta tener ficha): el primero para servidores, el segundo para clientes.
- Identificar al cliente por una cabecera falsificable: usa IP o clave de API autenticada.

## Profundiza en el curso

- [rate limiting a fondo](../../../08-patrones/06-rate-limiting)
- [429/503 y `Retry-After`](../../../15-diseno-avanzado/10-http-backpressure)
- [límite por tenant en `jobq`](../../../proyecto-final)
- [evitar un cerrojo global](../../../13-rendimiento/03-contador-fragmentado)

## Prueba tú

1. Cambia a un limitador por IP usando `r.RemoteAddr`.
2. Añade `Wait(ctx)` para un cliente que quiere respetar una cuota ajena.
3. Exporta cuántos rechazos hay por cliente como métrica.
