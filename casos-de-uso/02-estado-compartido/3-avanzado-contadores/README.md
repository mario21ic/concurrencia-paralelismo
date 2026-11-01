# Caso 2.3 — Métricas de alta frecuencia: no compartir  ·  `avanzado`

> **Problema:** 8 goroutines incrementan el MISMO contador sin parar. Un mutex funciona; un atomic es más rápido; pero ambos se disputan **una línea de caché**. Lo más rápido es no compartir: cada goroutine suma en su propio fragmento y se suman al leer.

## Qué features usa y por qué

| Feature de Go | Papel en este caso |
|--|--|
| `sync.Mutex` | referencia: correcto, pero serializa |
| `atomic.Int64` compartido | sin cerrojo, pero la línea de caché «rebota» entre núcleos |
| Contadores fragmentados | uno por goroutine; el total es la suma al leer |
| `cpu.CacheLinePad` | relleno para que dos fragmentos no compartan línea de caché (*false sharing*) |

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ Contadores/métricas con **mucha** escritura y lectura ocasional (el total puede ser aproximado en un instante).
- ✅ Ya mediste (pprof) que la contención en ese contador importa.
- ❌ Necesitas lecturas exactas y atómicas en cada instante (la suma no es una instantánea).
- ❌ Contención baja: un atomic o un mutex bastan; no compliques.

## Cómo ejecutarlo

```bash
go run .
go test -race -v .
```

## Qué observar (salida real, Apple M1, Go 1.26.5)

```
8 goroutines, 16000000 incrementos (ns por incremento; menos es mejor)
  mutex                                89.52 ns
  atomic.Int64 compartido              36.04 ns
  fragmentado (uno por goroutine)       1.47 ns
```

## Errores típicos

- Fragmentar **sin** relleno: dos contadores contiguos comparten línea y el *false sharing* te devuelve al problema.
- Optimizar sin medir: con pocos hilos las tres versiones empatan.
- Usar el id de goroutine como índice sin acotarlo al nº de fragmentos.

## Profundiza en el curso

- [el mismo experimento en profundidad](../../../13-rendimiento/03-contador-fragmentado)
- [qué es el *false sharing* y su coste](../../../13-rendimiento/02-false-sharing)
- [benchmark de contadores](../../../07-atomic/05-benchmark-contador)
- [fragmentar también mapas](../../../15-diseno-avanzado/06-mapa-sharded)

## Prueba tú

1. Quita el relleno (`cpu.CacheLinePad`) y mide cuánto empeora.
2. Prueba con 2 y con 16 goroutines: ¿cuándo empiezan a importar las diferencias?
3. Usa `sync.Pool` o variables por goroutine y compara.
