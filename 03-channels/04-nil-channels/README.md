# Ejercicio 4: channels `nil`

## Teoría

`var ch chan int` es `nil`. Sobre un channel nil:

| Operación | Resultado |
|--|--|
| `ch <- v` | bloquea para siempre |
| `<-ch` | bloquea para siempre |
| `close(ch)` | **panic** |
| `len(ch)`, `cap(ch)` | 0 |

Parece inútil, pero en un `select` un caso con channel nil **nunca se elige**. Se usa para **desactivar dinámicamente un caso**.

**Problema clásico:** fusionar dos channels con `select`. Cuando uno se cierra, su caso queda siempre listo (devuelve cero, `ok=false`) y el bucle gira consumiendo CPU. **Solución:** al detectar el cierre, asignar `nil` a esa variable; el caso deja de competir. El bucle termina cuando ambos son nil.

## Qué hace el programa

1. Comprueba que recibir de nil bloquea (se sale por timeout).
2. Fusiona dos channels que emiten a ritmos distintos, poniéndolos a nil al cerrarse.

```bash
go run .
```

## Resultado de referencia

```
nilCh == nil: true len: 0 cap: 0
recibir de nil: bloqueado (salimos por timeout)
fusionado: 1
fusionado: 100
fusionado: 200
fusionado: 2
fusionado: 3
```

El orden entre channels es no determinista; solo se garantiza el de cada uno.

## Conclusiones

- Nil en select = caso deshabilitado.
- Sin ese truco, `a` cerrado produciría una avalancha de `v=0, ok=false`.
- Es la base de `merge`/fan-in correcto (módulo 8).

## Ejercicios propuestos

1. Quita las asignaciones `a = nil` / `b = nil`. Añade un contador y observa cómo el bucle gira sin parar.
2. Usa el mismo truco para habilitar un caso de envío solo cuando haya un valor pendiente (`var salida chan<- int`).
3. Reescribe `fusionar` para N channels usando `reflect.Select` y compara la claridad.
