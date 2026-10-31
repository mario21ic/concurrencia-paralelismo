# Ejercicio 7: componer operaciones seguras no es seguro

## Teoría

Una estructura puede tener **todos sus métodos protegidos por un mutex** y aun así producir resultados incorrectos si el *llamador* encadena dos de ellos:

```go
if !set.Contiene(42) {   // ← operación 1 (atómica)
    set.Agregar(42)      // ← operación 2 (atómica)
}                        // pero la secuencia NO lo es
```

Entre la comprobación y la acción, otra goroutine puede cambiar el estado. Es un *race condition* lógico: **`-race` no lo detecta**, porque cada acceso a memoria está sincronizado. El detector solo encuentra *data races*, no errores de atomicidad.

Es el mismo problema de:
- "si no existe el archivo, créalo" (TOCTOU),
- "si hay saldo, retira",
- `len(ch) < cap(ch)` y luego enviar,
- `m.Load` + `m.Store` en `sync.Map` (usa `LoadOrStore`).

### Solución

Ofrecer **operaciones compuestas** que ejecuten la lectura y la escritura dentro de **una sola sección crítica**:

```go
func (c *Conjunto) AgregarSiNoExiste(k int) bool {
    c.mu.Lock()
    defer c.mu.Unlock()
    if c.m[k] { return false }
    c.m[k] = true
    return true
}
```

Criterio de diseño: expón las operaciones que **el negocio necesita como unidad** (`AgregarSiNoExiste`, `RetirarSiHaySaldo`, `GetOrCreate`), no las primitivas.

## Qué hace el programa

Hace 500 rondas; en cada una 100 goroutines arrancan a la vez e intentan ser "las primeras" en insertar la clave 42. Cuenta en cuántas rondas hubo más de una que creyó serlo.

- Versión mala: `Contiene` + `Agregar` (con `runtime.Gosched()` para ampliar la ventana).
- Versión buena: `AgregarSiNoExiste`.

```bash
go run .
go run -race .     # sin warnings: no hay data race, pero sí error lógico
```

## Resultado de referencia

```
check-then-act:    474/500 rondas con más de un "primero" (peor ronda: 87 a la vez)
operación atómica: 0/500 rondas incorrectas
```

(En otra ejecución: 312/500. El resultado varía; la versión atómica siempre da 0.)

## Conclusiones

- La versión con dos operaciones atómicas falla en la mayoría de las rondas, con picos de decenas de goroutines pensando que eran las primeras.
- `go run -race` **no** da avisos en la versión mala: cada acceso está protegido.
- La corrección depende de la **granularidad de la sección crítica**, no de que "todo tenga mutex".
- El `runtime.Gosched()` solo amplifica la ventana para la demostración; sin él el fallo es menos frecuente pero igual de real (y más difícil de reproducir en producción).

## Ejercicios propuestos

1. Quita el `runtime.Gosched()` y observa cuántas rondas siguen fallando.
2. Convierte `Cuenta` del ejercicio 1 para exponer `RetirarSiHaySaldo` y muestra el mismo bug con `Saldo()` + `Retirar()`.
3. Repite el problema con `sync.Map` (`Load` + `Store`) y arréglalo con `LoadOrStore`.
