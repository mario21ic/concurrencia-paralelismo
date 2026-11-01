# Caso 6.2 — Contar palabras: map y reduce  ·  `intermedio`

> **Problema:** Contar la frecuencia de cada palabra en 4 millones de palabras. La tentación es un mapa compartido con mutex; pero así los workers se pasan el tiempo esperándose.

## Qué features usa y por qué

| Feature de Go | Papel en este caso |
|--|--|
| `MutexGlobal` | un mapa y un cerrojo: concurrente pero **sin paralelismo útil** (contención) |
| `LocalYFusion` | MAP: un mapa local por worker; REDUCE: fusionar los parciales |
| Trozos contiguos | cada worker procesa su porción del corpus |
| `maps.Equal` en el test | comprobar que las tres versiones dan lo mismo |

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ Agregaciones (contar, sumar, máximos) sobre datos grandes.
- ✅ Siempre que el «reduce» sea mucho más barato que el «map».
- ❌ Si el resultado es enorme (millones de claves distintas), la fusión se vuelve el cuello de botella (la mejora cae); hay que fusionar en paralelo o por fragmentos.
- ❌ Si el dato cabe y es rápido de contar, el secuencial basta.

## Cómo ejecutarlo

```bash
go run .
go test -race -v .
```

## Qué observar (salida real, Apple M1, Go 1.26.5)

```
4000000 palabras, vocabulario de 5000, 8 workers (mejor de 5)
  secuencial            65.123834ms
  mapa compartido+mutex 637.373209ms (9.8× el secuencial)
  local + fusión        19.425166ms (3.4× más rápido que el secuencial)
```

## Errores típicos

- **Proteger todo con un mutex y pensar que ya es paralelo**: aquí fue 9.8× más lento que no paralelizar.
- Fusionar los mapas en una goroutine mientras otras aún escriben (carrera).
- Reutilizar el mapa `parciales[0]` como total y olvidar que ya no es «de solo lectura».
- Olvidar que hay que particionar por **datos**, no por clave (así no hay conflictos).

## Profundiza en el curso

- [map-reduce a fondo, con 2 M de claves](../../../13-rendimiento/06-map-reduce)
- [la versión con channels](../../../03-channels/09-miniproyecto-contador-palabras)
- [por qué el mutex serializa](../../../06-sync/01-mutex)
- [reducir contención](../../../13-rendimiento/03-contador-fragmentado)
- [el perfil de mutex mostraría la contención](../../../12-profiling/02-mutex-y-block)

## Prueba tú

1. Mide con un vocabulario de 5 000 000 de palabras distintas: ¿cuánto se erosiona la ganancia?
2. Fusiona los parciales en paralelo (árbol de fusiones).
3. Usa el perfil de mutex (`runtime.SetMutexProfileFraction`) sobre `MutexGlobal`.
