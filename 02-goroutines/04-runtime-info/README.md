# Ejercicio 4: paquete `runtime` y stack dinámico

## Teoría

| Función | Qué devuelve / hace |
|--|--|
| `runtime.NumCPU()` | núcleos lógicos disponibles para el proceso |
| `runtime.GOMAXPROCS(n)` | fija el número de P y devuelve el valor anterior; `n < 1` solo consulta |
| `runtime.NumGoroutine()` | goroutines existentes ahora (útil para detectar leaks) |
| `runtime.Gosched()` | cede el P voluntariamente |
| `runtime.ReadMemStats` | estadísticas de memoria, incluida `StackInuse` |

**GOMAXPROCS:**
- Por defecto es el número de núcleos lógicos.
- Desde Go 1.25, en Linux también respeta el **límite de CPU del cgroup** (p. ej. `resources.limits.cpu` en Kubernetes) y se ajusta si cambia. Antes había que usar `automaxprocs` de Uber.
- Se puede forzar con la variable de entorno `GOMAXPROCS=n` o con `runtime.GOMAXPROCS(n)`.

**Stack dinámico:** cada goroutine arranca con ~2 KB. Si necesita más, el runtime reserva un stack el doble de grande y copia el contenido; si luego sobra, el GC puede reducirlo. Por eso hay goroutines baratas, y por eso no puedes guardar punteros al stack fuera de Go.

## Qué hace el programa

```bash
go run .
GOMAXPROCS=2 go run .
```

1. Imprime información de CPU y goroutines.
2. Lanza 10 goroutines bloqueadas y vuelve a contar.
3. Ejecuta una recursión profunda en una goroutine y mide `StackInuse` antes y después.

## Resultado de referencia

```
NumCPU:         8
GOMAXPROCS(0):  8
NumGoroutine:   1
NumGoroutine con 10 bloqueadas: 11

StackInuse antes: 288 KB, con recursión profunda: 4384 KB

# con GOMAXPROCS=2 go run .
GOMAXPROCS(0):  2
```

## Conclusiones

- `NumGoroutine()` empieza en 1 (la de `main`) y es la herramienta más simple para sospechar de leaks: si crece sin parar, algo no termina.
- Una sola goroutine con recursión profunda consumió unos 4 MB de stack: crecer es automático pero no gratis.
- `NumCPU` es el hardware; `GOMAXPROCS` es cuántos P usa Go (puede ser menor).

## Ejercicios propuestos

1. Crea un contenedor Docker con `--cpus=2` y compara `NumCPU` y `GOMAXPROCS` (Go ≥ 1.25).
2. Escribe un test que verifique que `NumGoroutine()` vuelve al valor inicial al terminar.
3. Prueba `GOMAXPROCS=1 go run .` y razona qué cambia en los ejercicios 1 y 2 del módulo 1.
