# Ejercicio 1: `WithCancel` y el árbol de contextos

## Teoría

`ctx, cancel := context.WithCancel(parent)` devuelve un hijo de `parent` y una función para cancelarlo.

- Al llamar a `cancel()`: `ctx.Done()` se **cierra** y `ctx.Err()` pasa a `context.Canceled`.
- **Hacia abajo:** cancelar un contexto cancela a todos sus descendientes.
- **Nunca hacia arriba ni hacia los lados:** cancelar un hijo no afecta al padre ni a los hermanos.
- `cancel` es **idempotente**: se puede llamar varias veces.
- `Done()` cerrado equivale a una señal de difusión (como `close(done)` del módulo 3): despierta a todas las goroutines que esperan.
- Siempre `defer cancel()`: si no se llama, el hijo queda registrado en el padre hasta que este termine (fuga de recursos). `go vet` detecta la omisión.

```
padre ─┬─ hijoA   cancelar hijoA -> solo hijoA
       └─ hijoB   cancelar padre -> padre, hijoA y hijoB
```

## Qué hace el programa

1. Crea padre, hijoA, hijoB.
2. Cancela hijoA y comprueba que padre y hijoB siguen vivos.
3. Cancela el padre con tres goroutines escuchando; las tres despiertan.
4. Llama varias veces a `cancel` y verifica que `Done()` sigue cerrado.

```bash
go run .
```

## Resultado de referencia

```
antes: padre.Err = <nil> | hijoA.Err = <nil>
tras cancelar hijoA: hijoA.Err = context canceled | padre.Err = <nil> | hijoB.Err = <nil>
hijoA cancelado: context canceled
hijoB cancelado: context canceled
padre cancelado: context canceled
padre.Done() sigue cerrado: true
```

## Conclusiones

- El árbol de contextos modela la dependencia entre tareas: una petición cancelada cancela todo el trabajo que lanzó.
- Los mensajes se ordenan antes de imprimir porque el orden de despertar de las goroutines no está definido.

## Ejercicios propuestos

1. Quita `defer cancelB()` y ejecuta `go vet`. Lee el aviso.
2. Dibuja el árbol de contextos de un servidor HTTP con tres llamadas a servicios internos por petición.
3. Escribe un test que verifique que una goroutine termina en menos de 100 ms tras cancelar.
