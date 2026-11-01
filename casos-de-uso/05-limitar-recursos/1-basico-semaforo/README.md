# Caso 5.1 — Semáforo: máximo N a la vez  ·  `básico`

> **Problema:** Hay que descargar 20 ficheros, pero el servidor solo tolera 5 conexiones simultáneas (o la memoria da para 5). Lanzar 20 goroutines sin freno lo saturaría.

## Qué features usa y por qué

| Feature de Go | Papel en este caso |
|--|--|
| `make(chan struct{}, N)` | semáforo: enviar = coger plaza (bloquea si no hay), recibir = soltarla |
| `semaphore.Weighted` | semáforo con **pesos** (cada tarea ocupa más o menos) y cancelable con `ctx` |
| coger la plaza **antes** de `go` | evita crear 20 goroutines de golpe (solo se crean cuando hay sitio) |

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ Un recurso externo con límite de conexiones; trabajos que consumen mucha memoria; evitar agotar descriptores de fichero.
- ✅ `Weighted`: cuando las tareas no pesan igual.
- ❌ Si ya usas `errgroup`, `SetLimit` hace lo mismo (caso 1.2).
- ❌ Limitar **tasa** (por segundo) no es lo mismo: caso 5.3.
- ❌ Una cola con N workers fijos suele ser más clara que N plazas (caso 5.2).

## Cómo ejecutarlo

```bash
go run .
go test -race -v .
```

## Qué observar (salida real, Apple M1, Go 1.26.5)

```
20 tareas de 50 ms con distintos límites (tiempo ideal = ⌈20/límite⌉ × 50 ms):
  límite  1 → 1.02s  máx. simultáneas observadas: 1
  límite  2 → 510ms  máx. simultáneas observadas: 2
  límite  5 → 200ms  máx. simultáneas observadas: 5
  límite 10 → 100ms  máx. simultáneas observadas: 10
  límite 20 → 50ms   máx. simultáneas observadas: 20
con pesos [60 30 50 20 40] y presupuesto 100: pico de memoria en uso = 90
```

## Errores típicos

- Soltar la plaza **sin `defer`**: un pánico o un `return` temprano la pierde y el semáforo se «seca».
- Coger la plaza **dentro** de la goroutine: se crean todas las goroutines igualmente.
- Un semáforo de 1 plaza usado como mutex y un mutex usado como semáforo: elige la herramienta por la intención.
- `Weighted.Acquire` con un peso mayor que el total: espera para siempre (devuelve error con `ctx`).

## Profundiza en el curso

- [semáforos en profundidad](../../../08-patrones/05-semaforo)
- [`SetLimit`](../../../10-errores/01-errgroup)
- [pool de workers](../../../08-patrones/01-worker-pool)
- [ver quién espera una plaza](../../../12-profiling/03-goroutines-bloqueadas)

## Prueba tú

1. Mide con `-race` que `Monitor.Maximo` nunca supera el límite.
2. Haz que `ConChannel` acepte `ctx` y devuelva error si se cancela.
3. Prueba `ConPeso` con una tarea de peso 100 y otras de 10.
