# Ejercicio 5: timers desde Go 1.23

## Teoría

Go 1.23 cambió la implementación de `Timer` y `Ticker` (si `go.mod` declara `go 1.23` o más):

| Antes (≤1.22) | Ahora (≥1.23) |
|--|--|
| `t.C` tenía buffer 1 | `t.C` es síncrono: `cap=0`, `len=0` |
| Tras `Stop`/`Reset` podía quedar un valor viejo en `t.C` | **Imposible**: tras `Stop`/`Reset` ninguna recepción devuelve un valor anterior |
| Había que "drenar" `t.C` antes de `Reset` | Se puede llamar a `Reset` directamente |
| Un timer sin `Stop` no se recolectaba hasta expirar | Se recolecta en cuanto no hay referencias |
| Muchos `time.After(1h)` en un bucle ⇒ fuga temporal | Ya no |

Código viejo que drenaba el channel:

```go
if !t.Stop() {
    select { case <-t.C: default: }
}
t.Reset(d)
```

sigue funcionando, pero ya no es necesario.

Atención: como `len(t.C)` es siempre 0, el truco de mirar `len(t.C)` para saber si expiró **dejó de funcionar**.

Para volver al comportamiento previo: `GODEBUG=asynctimerchan=1` o bajar la versión en `go.mod`.

## Qué hace el programa

1. Imprime `cap` y `len` de `t.C` con el timer ya expirado.
2. Hace `Reset(100ms)` sobre un timer expirado y mide cuánto espera realmente.
3. `Stop` sobre un timer expirado no leído y comprueba que no queda valor.
4. Crea un millón de timers de 1 h sin `Stop` ni referencia y mide el heap tras `runtime.GC()`.

```bash
go run .
```

## Resultado de referencia

```
cap(t.C)=0 len(t.C)=0
tras Reset(100ms) esperó 100ms (no recibió el tick viejo)
Stop devuelve (expiró y nadie leyó): true
no hay valor viejo en t2.C
heap antes: 0.2 MB, tras 1M timers sin Stop + GC: 0.2 MB
```

## Conclusiones

- `Reset` después de expirar espera el nuevo plazo completo; antes de 1.23 habría recibido el tick viejo al instante.
- `Stop` devuelve `true` porque el valor no se había consumido, y garantiza que **no** se entregará después.
- Un millón de timers sin referencia no dejó rastro en el heap tras el GC. En Go ≤1.22 se habrían mantenido hasta expirar (1 hora).
- Nota: el punto 4 solo demuestra el comportamiento actual; para ver la diferencia hay que compararlo con `GODEBUG=asynctimerchan=1` y/o una versión antigua del compilador.

## Ejercicios propuestos

1. Ejecuta con `GODEBUG=asynctimerchan=1 go run .` y compara cada línea.
2. Cambia el `go` de `go.mod` a `1.22`, ejecuta y compara (recuerda restaurarlo).
3. Reescribe un reinicio de timer antiguo con drenado usando el `Reset` simple.
