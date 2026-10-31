# Ejercicio 2: `errors.Join` y errores múltiples

## Teoría

Un error puede envolver **uno** (`Unwrap() error`) o **varios** (`Unwrap() []error`). Dos formas de crear los segundos:

```go
errors.Join(e1, e2, e3)                    // mensajes separados por salto de línea
fmt.Errorf("fallo: %w y %w", e1, e2)       // varios %w (Go 1.20+)
```

Propiedades de `errors.Join`:
- Descarta los `nil`; si todos son `nil`, devuelve `nil` (así `return errors.Join(errs...)` es seguro).
- `errors.Is(err, X)` y `errors.As(err, &T)` recorren **todo el árbol**.
- El texto es la concatenación de los mensajes, uno por línea.

### Cuándo agregar y cuándo cortar

| Quieres... | Haz... |
|--|--|
| Validar un formulario y mostrar **todos** los problemas | acumular y `Join` |
| Cerrar varios recursos aunque alguno falle | acumular y `Join` |
| Ejecutar tareas independientes y dar un informe | slice de errores + `Join` |
| Operación atómica: si una parte falla, el resto sobra | fallar rápido (`errgroup`) |

### Errores concurrentes sin mutex

Cada goroutine escribe en **su propia posición** de un `[]error` preasignado; tras `wg.Wait()` se hace `errors.Join(errs...)`. Es seguro (índices distintos + happens-before de `Wait`), conserva el **orden determinista** y no necesita locks ni channel.

### Tipos de error propios

Los errores con campos (`*ErrValidacion`) se recuperan con `errors.As`, aunque estén dentro de un `Join` y de varios `%w` anidados. Los centinelas (`ErrNoEncontrado`) se detectan con `errors.Is`.

## Qué hace el programa

1. `Validar` acumula todos los errores de un usuario.
2. `Is` y `As` atraviesan el `Join`.
3. `Join(nil, nil) == nil`.
4. `fmt.Errorf` con dos `%w`.
5. Cuatro comprobaciones concurrentes cuyos fallos se unen.

```bash
go run .
go test -race -timeout 60s .
```

## Resultado de referencia

```
errores acumulados:
nombre: obligatorio
email: formato inválido
edad: fuera de rango

Is(ErrNoEncontrado): true
As(*ErrValidacion):  true -> id: vacío
Join(nil, nil) == nil: true
varios %w -> Is: true | hojas: 2

4 comprobaciones, 3 fallaron:
disco lleno
base de datos: no encontrado
sin red
¿alguna es ErrNoEncontrado? true
```

## Conclusiones

- Un solo valor `error` transporta todos los fallos y sigue funcionando con `Is`/`As`.
- El orden de los errores es el de los índices, no el de llegada: reproducible.
- `aplanar` devuelve solo las **hojas**; sirve para contar o registrar cada error por separado.

## Ejercicios propuestos

1. Implementa `Cerrar()` que cierre varios recursos y devuelva un `Join` de los fallos (con `defer`).
2. Haz que un error propio implemente `Unwrap() []error` y comprueba `Is`.
3. Serializa un `Join` a JSON (lista de mensajes) para una API.
