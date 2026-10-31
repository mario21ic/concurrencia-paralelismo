# Ejercicio 5: `context.WithValue`

## Teoría

`WithValue(parent, key, val)` devuelve un hijo que, al consultar `Value(key)`, devuelve `val`; si no, delega al padre (búsqueda ascendente por la cadena).

### Buenas prácticas

1. **Clave de tipo propio y no exportado** (`type claveCtx int`), nunca `string` ni tipos básicos: dos paquetes que usen `"usuario"` se pisan sin avisar.
2. **Funciones de acceso tipadas** (`ConUsuario(ctx, u)` / `Usuario(ctx)`): ocultan la clave y la conversión de tipo.
3. **Solo datos de ámbito de petición** que atraviesan APIs: ID de petición/traza, usuario autenticado, token de idioma.
4. **No** para parámetros de funciones, dependencias (`*sql.DB`, loggers por defecto…) ni opciones: es tipado dinámico y oculta el contrato; un parámetro explícito es más claro y verificable.
5. Los valores son **inmutables**: un hijo puede *sombrear* un valor, pero el padre no cambia.
6. La búsqueda es lineal por la cadena: no uses contextos como almacén general.

Reglas de decisión:

| Pregunta | Si es sí → |
|--|--|
| ¿La función no funciona sin ese dato? | parámetro explícito |
| ¿Es del ciclo de vida de la petición y lo consumen capas intermedias que no lo conocen? | contexto |

## Qué hace el programa

1. Define claves tipadas y accesores.
2. Propaga ID de petición y usuario por una cadena de funciones.
3. Sombrea el usuario en un hijo sin tocar el padre.
4. Muestra la colisión con claves `string` y cómo la evitan los tipos distintos.

```bash
go run .
```

## Resultado de referencia

```
valores propagados por la cadena de llamadas:
  [capaProfunda] id="req-42" usuario="ana"
hijo (usuario sombreado):
  [capaProfunda] id="req-42" usuario="admin"
padre intacto:
  [capaProfunda] id="req-42" usuario="ana"
claves string colisionan: del paquete B
claves de tipos distintos: A B
clave ausente: <nil>
```

## Conclusiones

- Con claves de tipos distintos no hay colisión posible entre paquetes.
- Con `string`, el segundo valor se impone silenciosamente: el linter `staticcheck` (SA1029) avisa de esto.
- El valor viaja sin ser parámetro: cómodo, pero también invisible. Úsalo con mesura.

## Ejercicios propuestos

1. Crea un middleware HTTP que inserte un ID de petición en el contexto y un `slog.Logger` que lo incluya.
2. Revisa tu código: ¿hay algún `WithValue` que debería ser un parámetro?
3. Mide el coste de `Value` con una cadena de 1, 10 y 100 contextos anidados.
