# Caso 1.2 — Lote con parada al primer error y límite  ·  `intermedio`

> **Problema:** Validar 20 ficheros contra un servicio externo. Si uno es inválido, seguir con los demás es trabajo perdido; y lanzar los 20 a la vez puede saturar al servicio.

## Qué features usa y por qué

| Feature de Go | Papel en este caso |
|--|--|
| `errgroup.WithContext` | el primer error **cancela** el contexto; `Wait()` devuelve ese error |
| `g.SetLimit(n)` | a lo sumo `n` en vuelo; `g.Go` **bloquea** cuando se alcanza (contrapresión al lanzar) |
| `ctx.Done()` dentro de la tarea | la cooperación: cada tarea debe mirar el contexto para poder abandonar |

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ Hay un resultado «todo o nada» (un lote que debe ser válido entero).
- ✅ Quieres limitar la concurrencia sin escribir un semáforo.
- ✅ Cada tarea puede **cancelarse** (usa `ctx` o es corta).
- ❌ Quieres **todos** los errores, no solo el primero → recoge en un slice con mutex, o `errors.Join` (módulo 10).
- ❌ Las tareas no respetan el contexto: la cancelación no sirve de nada (solo evita lanzar las siguientes).

## Cómo ejecutarlo

```bash
go run .
go test -race -v .
```

## Qué observar (salida real, Apple M1, Go 1.26.5)

```
error: f02: formato inválido
tardó 50ms (sin cancelar habría sido ≈ 2.5s) | máx. simultáneas: 4 | vieron la cancelación: 19 (en curso o por empezar)
```

## Errores típicos

- Ignorar el `ctx` devuelto por `WithContext` y usar el de fuera: la cancelación no llega a las tareas.
- Tareas que no miran `ctx.Done()`: «cancelan» solo cuando terminan solas.
- Lanzar con `g.Go` miles de tareas **sin** `SetLimit`: miles de goroutines pidiendo recursos a la vez.
- Creer que `Wait` devuelve todos los errores: devuelve **el primero**.

## Profundiza en el curso

- [`errgroup` a fondo](../../../10-errores/01-errgroup)
- [cómo funciona la cancelación](../../../05-context/01-withcancel)
- [tareas que cooperan con `ctx`](../../../05-context/03-cancelacion-cooperativa)
- [concurrencia estructurada: nada sobrevive a la función](../../../15-diseno-avanzado/09-apis-concurrentes)

## Prueba tú

1. Quita `SetLimit` y observa el máximo de simultáneas.
2. Hazlo con `sync.WaitGroup` + `context.WithCancelCause` a mano y compara el código.
3. Haz que falle el elemento 15 en vez del 2: ¿cuánto tarda ahora en volver?
