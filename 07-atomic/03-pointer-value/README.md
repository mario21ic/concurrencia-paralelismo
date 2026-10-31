# Ejercicio 3: `atomic.Pointer` y `atomic.Value`

## Teoría

Dos `atomic` **no** forman una operación atómica. Si una configuración tiene varios campos relacionados (`Nivel` y `Limite`) y los actualizas por separado, un lector puede ver `Nivel` nuevo con `Limite` viejo.

### Solución: *copy-on-write* con un puntero atómico

```go
var cfg atomic.Pointer[Config]

// lector (sin locks, nunca bloquea)
c := cfg.Load()
usar(c.Nivel, c.Limite)

// escritor: construye una Config NUEVA y la publica entera
cfg.Store(&Config{Nivel: n, Limite: n * 10})
```

Reglas:

1. La estructura apuntada es **inmutable**: jamás modifiques `*cfg.Load()` in situ.
2. El lector toma **una foto** (`c := cfg.Load()`) y usa solo esa; no vuelvas a llamar a `Load()` por cada campo.
3. El GC libera la versión vieja cuando el último lector la suelta.
4. Para "leer-modificar-escribir" con varios escritores: bucle CAS (`CompareAndSwap(viejo, nuevo)`) o un mutex entre escritores.

Es el patrón de la recarga de configuración en caliente, tablas de enrutado, listas de permisos o flags de características.

### `atomic.Value` (anterior a los genéricos)

Guarda un `any`. Reglas que producen **panic**:

- `Store(nil)`.
- Almacenar un valor de **tipo distinto** al primero.

Prefiere `atomic.Pointer[T]`: seguro por tipos, sin conversión ni estos panics. `Value` tiene sentido con tipos que no son punteros (p. ej. un `[]string` directamente) o código anterior a Go 1.19.

## Qué hace el programa

Un escritor cambia la configuración sin parar mientras 4 lectores comprueban el invariante `Limite == Nivel*10` durante 200 ms:

- **A)** `Nivel` y `Limite` en dos `atomic.Int64`.
- **B)** Un `atomic.Pointer[Config]` con una `Config` inmutable.
- **C)** `CompareAndSwap` sobre el puntero.
- **D)** Los panics de `atomic.Value`.

```bash
go run .
```

## Resultado de referencia

```
dos atomic.Int64:    3300686 lecturas, 338207 INCOHERENTES
atomic.Pointer:      3948329 lecturas,      0 incoherentes

CAS con el puntero que leí:    true
CAS con un puntero ya obsoleto: false

Value.Load() sin Store: <nil>
Store de otro tipo:          -> panic: sync/atomic: store of inconsistently typed value into Value
Store(nil):                  -> panic: sync/atomic: store of nil value into Value
```

## Conclusiones

- Con dos atómicos, **~10 % de las lecturas** vieron un estado incoherente, aunque cada operación era atómica. Con el puntero, **ninguna**.
- Publicar una estructura inmutable convierte "varias escrituras" en una sola.
- `-race` **no** detecta el caso A: no hay data race, hay incoherencia lógica.
- El CAS sobre puntero falla si el puntero que usas ya fue reemplazado.

## Ejercicios propuestos

1. Añade un tercer campo a `Config` y verifica que el invariante se mantiene.
2. Implementa `Actualizar(f func(Config) Config)` con un bucle CAS para varios escritores.
3. Recarga la configuración desde un archivo JSON al recibir `SIGHUP` y publícala con `atomic.Pointer`.
4. Reescribe el lector del caso A tomando un mutex y compara rendimiento.
