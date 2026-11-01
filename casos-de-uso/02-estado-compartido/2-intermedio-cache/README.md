# Caso 2.2 — Caché con TTL y sin estampida  ·  `intermedio`

> **Problema:** 100 peticiones piden a la vez el MISMO dato que no está en caché. Sin cuidado, las 100 van a la BD (estampida / *thundering herd*). Queremos UNA consulta y que las otras 99 esperen su resultado.

## Qué features usa y por qué

| Feature de Go | Papel en este caso |
|--|--|
| `sync.RWMutex` | muchos lectores a la vez (`RLock`), un escritor en exclusiva (`Lock`): la caché se lee mucho |
| `singleflight.Group` | unifica las cargas simultáneas de una misma clave: una ejecuta, las demás reciben su resultado |
| Re-comprobar tras `Do` | otra goroutine pudo rellenar la caché mientras esperábamos |
| TTL con `time.Now()` | la entrada caduca; los errores **no** se cachean |

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ Lecturas ≫ escrituras y cargar el dato es caro (BD, red, cálculo).
- ✅ Muchas peticiones coinciden en la misma clave en ráfagas.
- ❌ Datos que cambian constantemente: la caché sirve datos viejos (usa TTL corto o invalidación).
- ❌ Si el dato cabe y no cambia nunca, un `sync.Once`/`OnceValue` es más simple.

## Cómo ejecutarlo

```bash
go run .
go test -race -v .
```

## Qué observar (salida real, Apple M1, Go 1.26.5)

```
100 peticiones simultáneas → 1 consulta(s) a la BD, en 100ms
petición siguiente (en caché) → 1.709µs, consultas totales: 1
```

## Errores típicos

- Mantener el cerrojo **mientras** se carga (bloquea a todos, incluso a claves distintas).
- No volver a mirar la caché dentro de `Do` (doble carga).
- Cachear errores: un fallo transitorio queda «pegado» hasta que caduque.
- Caché sin límite de tamaño: en producción añade expulsión (LRU) o límite.

## Profundiza en el curso

- [`RWMutex`](../../../06-sync/02-rwmutex)
- [`singleflight` a fondo](../../../10-errores/04-singleflight)
- [caché con TTL](../../../07-atomic/07-miniproyecto-cache-ttl)
- [`sync.Map`: cuándo sí](../../../06-sync/05-sync-map)
- [mapa fragmentado para alta contención](../../../15-diseno-avanzado/06-mapa-sharded)

## Prueba tú

1. Sustituye `RWMutex` por `Mutex` y mide con 8 goroutines leyendo.
2. Añade un límite de entradas y expulsa la más antigua.
3. Usa `sync.Map` y observa qué pierdes (tipado, atomicidad de «leer o cargar»).
