# Ejercicio 3: `Once` y derivados

## Teoría

`sync.Once` ejecuta una función **exactamente una vez**, aunque muchas goroutines llamen a `Do` a la vez; las demás **esperan** a que termine la primera. Tras `Do`, todo lo que `f` escribió es visible (happens-before).

| API | Qué hace |
|--|--|
| `once.Do(f)` | ejecuta `f` una vez |
| `sync.OnceFunc(f) func()` | envuelve `f`; la llamada devuelta ejecuta `f` una sola vez |
| `sync.OnceValue(f) func() T` | como la anterior, y **cachea el valor** de retorno |
| `sync.OnceValues(f) func() (T1, T2)` | cachea dos valores, típicamente `(valor, error)` |

Los tres últimos son de Go 1.21 y son la forma moderna de escribir un singleton perezoso:

```go
var config = sync.OnceValue(cargarConfig)   // config() carga la primera vez
```

### Detalles que muerden

- **Un error cacheado se queda cacheado.** Con `OnceValues(func() (T, error))`, si falla la primera vez, devolverá ese error para siempre; no hay reintento. Si necesitas reintentar, usa un `Mutex` con tu propia lógica.
- **Pánico en `Once.Do`:** `Do` lo considera ejecutado; los siguientes `Do` no reintentan `f`. Deja el recurso a medio inicializar sin que nadie lo sepa.
- **Pánico en `OnceFunc`/`OnceValue`:** cada llamada posterior **vuelve a lanzar el mismo pánico**. Es más seguro: el fallo es visible.
- **No llames a `Do` dentro de `f` sobre el mismo `Once`**: deadlock.
- No copiar un `Once` usado.
- Los `init()` y variables globales se inicializan secuencialmente al arrancar; `Once` es para inicialización **perezosa** (cuando quizás no se use nunca, o es costosa).

## Qué hace el programa

1. 10 goroutines llaman a `Do`: `f` corre 1 vez.
2. `OnceValue`: la configuración se carga solo al usarla y una sola vez.
3. `OnceValues` con error: se cachea.
4. Pánico en `Once.Do` frente a `OnceFunc`.

```bash
go run .
```

## Resultado de referencia

```
Once.Do ejecutó f 1 vez (de 10 llamadas)
antes de usar config, cargas: 0
  (cargando configuración...)
config: {postgres://localhost/app} | cargas: 1
conexion(): base de datos no disponible  (x3)
intentos reales de conexión: 1 (para reintentar, usa un mutex propio)
Once.Do con panic recuperado: falló la inicialización
¿segundo Do ejecutó f? false
OnceFunc llamada 1 -> panic: boom
OnceFunc llamada 2 -> panic: boom
```

## Conclusiones

- La carga perezosa solo ocurre cuando se pide (0 cargas antes de usar).
- Tres llamadas a `conexion()` produjeron un único intento real.
- `OnceFunc` hace visible un fallo de inicialización en cada uso; `Once.Do` lo silencia.

## Ejercicios propuestos

1. Implementa un `Reintentable[T]` con mutex que reintente si la función devolvió error.
2. Usa `OnceValue` para cargar una expresión regular compilada y úsala desde 100 goroutines.
3. Provoca el deadlock llamando `once.Do` dentro de su propia `f`.
