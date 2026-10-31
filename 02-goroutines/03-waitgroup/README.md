# Ejercicio 3: `sync.WaitGroup` y `WaitGroup.Go`

## Teoría

`WaitGroup` es un contador:

| Método | Efecto |
|--|--|
| `Add(n)` | suma `n` al contador (antes de lanzar la goroutine) |
| `Done()` | resta 1 (equivale a `Add(-1)`) |
| `Wait()` | bloquea hasta que el contador llega a 0 |
| `Go(f)` (Go 1.25+) | `Add(1)`, lanza `f` en una goroutine y hace `Done()` al terminar |

Forma clásica:

```go
wg.Add(1)
go func() { defer wg.Done(); trabajo() }()
```

Forma moderna: `wg.Go(trabajo)`. Es más corta y evita olvidar el `Done` o poner el `Add` en el sitio equivocado.

### Errores típicos

1. **`Add` dentro de la goroutine:** `Wait` puede ejecutarse antes que cualquier `Add`, y retorna sin esperar. `go vet` lo avisa (`WaitGroup.Add called from inside new goroutine`).
2. **Copiar el `WaitGroup`** (pasarlo por valor): cada copia tiene su contador. Pásalo como `*sync.WaitGroup`.
3. **Contador negativo:** más `Done` que `Add` provoca `panic`.
4. **Panic sin recover:** mata todo el proceso, no solo la goroutine. Un `WaitGroup` no captura panics.
5. **Reutilizar** el grupo antes de que `Wait` retorne.

## Qué hace el programa

```bash
go run .                       # clásico y moderno + recuperación de panic
go run . -modo=add-dentro      # reproduce el bug (puede abortar el proceso)
```

> `go vet ./...` señalará `add-dentro`; es intencional.

## Resultado de referencia

```
clásico: 1000
moderno: 1000
error: panic recuperado: algo salió mal

add-dentro, 200 ejecuciones: Wait retornó antes de tiempo 192 veces, panic del runtime 6 veces
```

En otras ejecuciones el runtime aborta con `panic: sync: WaitGroup misuse: Add called concurrently with Wait` o `WaitGroup is reused before previous Wait has returned`; esos panics ocurren en una goroutine y **no** se pueden recuperar desde `main`, de modo que el proceso muere. El bug se manifiesta de varias formas, ninguna buena.

## Conclusiones

- Prefiere `wg.Go` en código nuevo (requiere Go ≥ 1.25).
- `Add` siempre **antes** de lanzar la goroutine, en la goroutine que lanza.
- Para convertir panics en errores, envuelve la tarea con `defer recover()` (función `seguro`).
- `WaitGroup` no devuelve errores. Para eso, `errgroup` (módulo 10).

## Ejercicios propuestos

1. Pasa el `WaitGroup` por valor a una función y ejecuta `go vet`.
2. Quita el `recover` de `seguro` y observa que el proceso muere.
3. Haz que `seguro` también capture el stack con `debug.Stack()`.
