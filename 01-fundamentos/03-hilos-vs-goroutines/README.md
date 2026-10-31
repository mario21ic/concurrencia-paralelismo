# Ejercicio 3: procesos, hilos e hilos "verdes"

## Teoría

| | Proceso | Hilo de SO | Goroutine |
|--|--|--|--|
| Memoria | espacio propio (aislado) | comparte la del proceso | comparte la del proceso |
| Stack inicial | n/a | ~1–8 MB (fijo/reservado) | ~2 KB (crece y se reduce) |
| Planificador | SO | SO | runtime de Go |
| Cambio de contexto | muy caro | caro (kernel) | barato (espacio de usuario) |
| Creación | muy cara | cara (~decenas de µs) | ~1 µs |
| Límite práctico | cientos | miles | millones |

**Modelo M:N (G-M-P):** muchas **G** (goroutines) se multiplexan sobre pocos **M** (hilos de SO), y cada M necesita un **P** (contexto de ejecución) para correr código Go. Hay tantos P como `GOMAXPROCS`. El scheduler lo veremos en detalle en el módulo 14.

Las goroutines son *hilos verdes* planificados en espacio de usuario: no hay llamada al kernel para cambiar entre ellas.

## Qué hace el programa

Lanza `n` goroutines que quedan bloqueadas en un channel, mide el tiempo de creación y la memoria de stack usada.

```bash
go run .                # 100 000 goroutines
go run . -n 1000000     # un millón (necesitas ~2 GB libres)
```

## Resultado de referencia

```
goroutines vivas:      100000
tiempo de creación:    100ms (1.004µs por goroutine)
stack total:           204.9 MB
stack por goroutine:   ~2049 bytes
referencia: 100000 hilos de SO de 8 MB reservarían 781.2 GB (virtual)
```

## Conclusiones

- Una goroutine cuesta ~2 KB de stack y ~1 µs en crearse.
- 100 000 hilos de SO serían inviables; 100 000 goroutines caben en ~200 MB.
- "Barato" no es "gratis": una goroutine que nunca termina es un **leak** (lo trataremos en el módulo 11).

## Ejercicios propuestos

1. Prueba `-n 1000000` y observa memoria con `top` o Activity Monitor.
2. Haz que cada goroutine use un array local de 64 KB y observa cómo crece el stack.
3. Compara con crear hilos de SO en otro lenguaje (C/Python `threading`) y anota el límite en tu máquina.
