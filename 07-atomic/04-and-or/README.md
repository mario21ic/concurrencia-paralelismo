# Ejercicio 4: `And` y `Or` atómicos

## Teoría

Go 1.23 añadió `And` y `Or` a los tipos atómicos enteros (`Int32`, `Int64`, `Uint32`, `Uint64`, `Uintptr`) y las funciones `atomic.AndUint32(&x, m)`, `atomic.OrUint32(&x, m)`, etc. Aplican una operación a nivel de bits **atómicamente** y devuelven el valor **anterior**.

| Operación | Efecto sobre los bits |
|--|--|
| `x.Or(m)` | **activa** los bits de `m` |
| `x.And(m)` | **conserva** solo los bits de `m` |
| `x.And(^m)` | **apaga** los bits de `m` |

Casos de uso: conjuntos de banderas o permisos, un bitmap de "ranuras ocupadas", estados compuestos pequeños.

### Por qué el valor anterior importa

```go
ant := perms.Or(Ejecutar)
if ant&Ejecutar == 0 { /* YO fui quien lo activó */ }
```

Permite decidir quién hizo la transición, sin carreras.

### Antes de 1.23

Se simulaba con un bucle CAS (`for { v := x.Load(); if x.CompareAndSwap(v, v|m) { break } }`). `Or`/`And` son más simples y en algunas arquitecturas usan una única instrucción nativa.

### El error típico

```go
v := x.Load()
x.Store(v | bit)   // Load y Store son atómicos por separado, la secuencia NO
```

Dos goroutines leen el mismo `v`, cada una escribe `v | su_bit` y una **pisa** a la otra: se pierde un bit.

## Qué hace el programa

1. Muestra el efecto y el valor de retorno de `Or`/`And` sobre `Leer`/`Escribir`/`Ejecutar`.
2. Usa las funciones `OrUint32`/`AndUint32`.
3. 500 rondas con 32 goroutines que activan cada una **su** bit: versión `Load+Store` frente a `Or`.

```bash
go run .
```

## Resultado de referencia

```
Or(Leer|Escribir)  anterior=000 ahora=011
And(^Escribir)     anterior=011 ahora=001
¿activé Ejecutar yo?       true
¿lo activé de nuevo yo?    false (ya estaba activo)
OrUint32/AndUint32: 0010

Load+Store: 303/500 rondas perdieron bits (1085 bits perdidos en total)
Or atómico: 0/500 rondas con bits perdidos
```

## Conclusiones

- `Or` y `And` devuelven el valor **anterior**, útil para detectar transiciones.
- El mismo problema que en el módulo 6 (*check-then-act*): una secuencia de operaciones atómicas no es atómica. Aquí se pierden bits en el 60 % de las rondas.
- Los números exactos varían entre ejecuciones; la versión atómica siempre da 0.

## Ejercicios propuestos

1. Implementa un bitmap de 64 ranuras con `Uint64`: `Reservar()` encuentra un bit libre y lo toma con `Or` (¿cómo detectas que otro lo tomó antes?).
2. Reescribe `Or` con un bucle CAS y compara el rendimiento.
3. Usa `And(^Escribir)` y comprueba qué ocurre con el valor devuelto si el bit ya estaba apagado.
