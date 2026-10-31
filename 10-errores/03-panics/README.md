# Ejercicio 3: pánicos en goroutines

## Teoría

### Reglas del lenguaje

1. Un pánico **no recuperado** en cualquier goroutine termina **todo el proceso** (código de salida 2), aunque `main` esté sano.
2. `recover()` solo tiene efecto **llamado directamente desde un `defer`** de la goroutine que está en pánico.
3. Por eso `defer recover()` en `main` **no** protege de pánicos en otras goroutines.
4. Cada goroutine que pueda entrar en pánico debe tener su propia protección, normalmente en una **función envoltorio**.

### `PanicError`

Convertir el pánico en un error normal permite al código que espera (p. ej. `errgroup.Wait`) tratarlo como cualquier otro fallo, con la traza de pila (`debug.Stack()`) para depurar.

```go
func Capturar(f func() error) (err error) {
    defer func() {
        if r := recover(); r != nil {
            err = &PanicError{Valor: r, Stack: debug.Stack()}
        }
    }()
    return f()
}
```

- `Unwrap()` devuelve el error original si se hizo `panic(err)`, así `errors.Is` sigue funcionando.
- `debug.Stack()` se llama **dentro** del `defer`, mientras el pánico sigue activo: así la traza incluye el punto del fallo.

### ¿Cuándo recuperar?

| Sí | No |
|--|--|
| frontera de una petición o tarea (un fallo no debe tumbar el servicio) | tragarse el pánico en silencio |
| pool de workers con tareas de terceros | para "arreglar" una violación de invariante |
| convertir en error y **registrar con traza** | en bibliotecas, ocultando pánicos del llamador |

Algunos pánicos **no** son recuperables: `fatal error: all goroutines are asleep`, `concurrent map writes`, desbordamiento de pila, falta de memoria.

### `errgroup` y pánicos

Con `golang.org/x/sync v0.24.0` (la versión de este repositorio) `errgroup` **no** recupera pánicos: se comprobó con una tarea que hace `panic("boom")` y el proceso murió. De ahí `GoSeguro`. Revisa la versión que uses.

## Qué hace el programa

```bash
go run .                 # protegido
go run . -modo=crash     # un pánico en una goroutine mata el proceso (exit status 2)
go run . -modo=main      # recover en main NO lo atrapa
go test -race -timeout 60s .
```

## Resultado de referencia

```
1) error: panic: assignment to entry in nil map | es pánico: true
   primera línea de la traza: goroutine 1 [running]:
2) errors.Is(err, errBase): true
3) Wait: panic: tarea 1 explotó | es pánico: true
4) función normal -> <nil>

# -modo=crash
panic: nadie me recupera
goroutine 19 [running]: ...
exit status 2

# -modo=main
panic: pánico en otra goroutine       <- el defer recover() de main no lo atrapa
```

## Conclusiones

- El pánico de una tarea del grupo llegó como error, canceló a las demás (con `WithContext`) y el proceso siguió vivo.
- Sin protección el proceso muere con código 2.
- `recover()` en `main` no ayuda: hay que proteger **dentro** de cada goroutine.

## Ejercicios propuestos

1. Haz que `GoSeguro` registre el pánico con `slog` incluyendo el `Stack`.
2. Escribe un middleware HTTP que recupere pánicos de los handlers y responda 500 (`net/http` ya lo hace por conexión, pero no registra como tú quieras).
3. Comprueba que `runtime.Goexit()` ejecuta los `defer` pero `recover()` devuelve `nil`.
4. Propaga el pánico a quien espera: captura en la goroutine y relánzalo en la goroutine que llama a `Wait` (como hacen `Prefetch`/`ParMap` del módulo 9).
