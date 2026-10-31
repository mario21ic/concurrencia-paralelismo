# Ejercicio 1: `errgroup`

## Teoría

`errgroup.Group` es un `WaitGroup` que además **recoge el primer error** y, con `WithContext`, **cancela** el trabajo restante.

| Elemento | Comportamiento |
|--|--|
| `g.Go(f)` | lanza `f` en una goroutine (bloquea si hay `SetLimit` y está lleno) |
| `g.TryGo(f)` | igual pero devuelve `false` en lugar de bloquear |
| `g.SetLimit(n)` | máximo `n` goroutines activas (`-1` = sin límite); debe llamarse antes de `Go` |
| `g.Wait()` | espera a todas; devuelve el **primer** error no nil |
| `errgroup.WithContext(ctx)` | contexto derivado que se cancela al primer error **o** cuando `Wait` retorna |

### Errores frecuentes

1. **Usar el `ctx` externo dentro de las tareas** (punto 3 del ejercicio): la cancelación nunca llega.
2. **Ignorar el contexto** en tareas largas: el error de una no detiene a las otras.
3. **Perder errores**: solo se devuelve el primero.
4. **Esperar que recupere pánicos**: no lo hace (ver ejercicio 3).
5. **Lanzar `g.Go` desde dentro del mismo grupo con `SetLimit` lleno**: puede producir deadlock.

### `errgroup` frente a `WaitGroup`

| | `WaitGroup` | `errgroup` |
|--|--|--|
| Espera | sí | sí |
| Error | no | primer error |
| Cancelación | no | con `WithContext` |
| Límite | no | `SetLimit` |

## Qué hace el programa

4 tareas de 50, 100, 150 y 200 ms; la primera falla.

1. Sin contexto: todas terminan, aunque la primera falló.
2. Con `WithContext`: las demás se cancelan al instante.
3. Bug: usar el contexto externo.
4. `SetLimit(3)` con 10 tareas.
5. `TryGo`.
6. Tres tareas fallan y se pierde información.

```bash
go run .
go test -race -timeout 60s .
```

## Resultado de referencia

```
1) sin contexto: err="tarea 1 falló", terminaron 4/4, tiempo=200ms (esperó a todas)
2) con contexto: err="tarea 1 falló", tiempo=50ms (las demás se cancelaron al instante)
   ctx tras Wait: context canceled (Wait siempre cancela el contexto derivado)
3) BUG (ctx externo): tiempo=210ms (nadie se enteró del error)
4) SetLimit(3): 10 tareas de 30ms -> máx. simultáneas=3, tiempo=150ms (ideal 120ms)
5) TryGo con hueco: true
   TryGo sin hueco: false
6) tres tareas fallaron; Wait devuelve solo el primero: fallo 1 (los otros dos errores se perdieron)
```

## Conclusiones

- Con contexto: 50 ms; sin contexto: 200 ms. La cancelación cuatriplica la rapidez de fallo.
- El bug del contexto externo es silencioso: no hay error ni aviso, solo desperdicio.
- `SetLimit(3)` con 10 tareas de 30 ms da 4 "olas" (≈120 ms ideal; 150 ms medidos incluyen overhead del scheduler).

## Ejercicios propuestos

1. Descarga 20 URLs (`httptest`) con `SetLimit(5)` y cancela todas ante el primer fallo.
2. Recoge **todos** los errores con un slice indexado y `errors.Join`, y compara con `Wait`.
3. Reproduce el deadlock de `g.Go` anidado con `SetLimit(1)`.
