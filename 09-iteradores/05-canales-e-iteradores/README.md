# Ejercicio 5: canales ↔ iteradores y APIs paginadas

## Teoría

### Convertir entre ambos mundos

| Conversión | Función | Atención |
|--|--|--|
| channel → iterador | `DeCanal(ch)` | un `break` deja de leer pero **no cierra ni vacía** el channel: el productor debe poder terminar por otra vía (`ctx`) o se quedará bloqueado |
| iterador → channel | `ACanal(ctx, seq)` | lanza una goroutine; hay que cancelar `ctx` si el consumidor abandona |

En `ACanal` el `range` sobre `seq` ocurre en la goroutine nueva, y es esa goroutine la que invoca `yield`: el contrato se cumple.

### Paginación perezosa: el caso estrella

Una API paginada devuelve los datos de a `N` elementos. Con un iterador:

```go
for item, err := range Elementos(ctx, pedirPagina) {
    if err != nil { ... }
    ...
    if suficiente { break }   // NO se piden más páginas
}
```

Ventajas:
- El consumidor ve un **flujo plano**, sin saber de páginas.
- **Perezoso:** la página siguiente solo se pide cuando se agota la actual → con un `break` ahorras llamadas de red.
- **Errores en el flujo:** `Seq2[T, error]` entrega el error como un elemento más y termina.
- **Cancelación:** el contexto cancelado se entrega como error (y se pasa a `pedir` para abortar la petición en curso).
- Sin goroutines ni channels: el coste es el de las llamadas de red.

Convención para `Seq2[T, error]`: tras entregar un error, el iterador **termina**. El consumidor decide si hace `break` o ignora.

## Qué hace el programa

1. `DeCanal` y `ACanal` (con cancelación).
2. API simulada de 10 páginas × 3 elementos (20 ms por página):
   - tomar solo 5 elementos,
   - recorrer todo,
   - error 503 en la tercera página,
   - plazo de 70 ms.

```bash
go run .
go test -race -count=3 .
```

## Resultado de referencia

```
DeCanal: [a b c]
ACanal: suma hasta el 30 = 60

tomé 5 elementos [0 1 2 3 4] pidiendo solo 2 de 10 páginas
recorrido completo: 30 elementos, 10 páginas pedidas

error tras 6 elementos: la API devolvió 503
con plazo de 70ms: 9 elementos y error "context deadline exceeded"
```

## Conclusiones

- Para 5 elementos solo se pidieron **2 de 10 páginas**: ahorro del 80 % de llamadas.
- Con el 503 el consumidor recibió los 6 elementos anteriores y luego el error.
- Con plazo de 70 ms se obtuvieron 3 páginas (9 elementos) y el contexto cancelado interrumpió la cuarta petición.

## Tests

`DeCanal`, `ACanal` sin fugas al cancelar, paginación perezosa (4 elementos ⇒ exactamente 2 páginas), recorrido completo (30 / 10), error a mitad (3 elementos y error) y contexto ya cancelado (0 páginas pedidas).

## Ejercicios propuestos

1. Combina `Elementos` con `ParMap` para procesar los elementos de la API en paralelo conservando el orden.
2. Haz que `Elementos` **prefetch**ee la página siguiente mientras el consumidor procesa la actual (usa `Prefetch` o una goroutine interna con las reglas del ejercicio 4).
3. Añade reintentos con *backoff* al pedir una página.
4. Implementa el mismo recorrido con channels y compara líneas de código y manejo de cancelación.
