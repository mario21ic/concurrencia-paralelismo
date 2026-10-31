# Ejercicio 9: diseño de APIs concurrentes y concurrencia estructurada

## Teoría

Una API concurrente tiene que responder, **en su firma o su documentación**, a cuatro preguntas:

1. **¿Quién lanza las goroutines?** (¿el llamante o la biblioteca?)
2. **¿Quién las detiene y cómo?** (`ctx`, `Close`, cierre de un channel)
3. **¿Cómo se entera el llamante de que han terminado y de sus errores?**
4. **¿Qué pasa si el llamante se desentiende** (sale antes, hace `break`, olvida cerrar)?

### Regla 1: deja que el llamante decida la concurrencia

> "Esa función podría ser síncrona; que sea quien llama quien decida si va en una goroutine." — Go Code Review Comments / Dave Cheney

| Diseño | Ventajas | Problemas |
|--|--|--|
| **`Run(ctx) error` bloqueante** (como `http.Server.Serve`, `errgroup`) | el llamante controla el `go`, el ctx y recibe el error; trivial de probar | el llamante debe lanzarlo |
| **Constructor que lanza una goroutine** (`NewX()` con `go` dentro) | cómodo a corto plazo | ciclo de vida oculto; **fuga por instancia** si no hay `Close`; difícil de probar; efectos al hacer `init()` |
| **`Iniciar` + `Close()` que ESPERA** | el componente *debe* tener hilos propios y ofrece un asa | hay que acordarse de llamar a `Close` (documéntalo; `defer`) |

### Regla 2: toda goroutine necesita una condición de salida y un dueño

Antes de escribir `go`, responde: **¿cuándo termina?** y **¿quién espera a que termine?** Las cuatro formas de salida: se acaba el trabajo, se cancela un `ctx`, se cierra un channel de entrada, o se llama a `Close`. Si no puedes decir cuál, es una fuga en potencia.

### Regla 3: channels en las APIs

- **Quien envía, cierra** (nunca el receptor); cerrar un channel dos veces o enviar a uno cerrado es un pánico.
- Todo envío o recepción que pueda bloquear debe poder **competir con `ctx.Done()`** (`select`).
- Devuelve `<-chan T` (solo lectura) y documenta si el channel se cierra y cuándo.
- Si el consumidor puede abandonar antes de tiempo, una goroutine que envíe a un channel sin buffer **se queda bloqueada para siempre** (fuga). Remedios: `ctx`, buffer suficiente, o…
- …**un iterador (`iter.Seq`)**: es **síncrono**, no hay goroutine que fugar, `break` termina el bucle (`yield` devuelve `false`). Si no necesitas paralelismo, es la API más segura. (Para paralelismo + iterador, `iter.Pull` o un channel interno.)

### Regla 4: `context.Context`

- Primer parámetro, nombre `ctx`; **no se guarda en un struct** (salvo tipos que representan una solicitud). `Run(ctx)` y `Iniciar(ctx, …)` lo reciben, no lo almacenan.
- `context.WithCancelCause` + `context.Cause` dicen **por qué** se canceló (aquí `ErrCerrado` o el primer error de una descarga).

### Concurrencia estructurada

> Una goroutine no debe sobrevivir a la función que la lanzó.

Como un `{ … }` de bloque: **todo lo que se abre dentro, se cierra dentro**. Aporta: sin fugas, errores propagados, cancelación en cascada, y que el resultado sea seguro de leer al volver (no hay carreras con goroutines que siguen escribiendo).

En Go no está en el lenguaje, se consigue con:

| Herramienta | Qué da |
|--|--|
| **`errgroup.WithContext`** (`golang.org/x/sync`) | espera a todas, **primer error** cancela las demás, `SetLimit(n)` acota la concurrencia |
| **`sync.WaitGroup.Go`** (Go 1.25) | lanza + cuenta en una línea; sin errores ni límites |
| **`context.WithCancelCause`** | cancelación + causa, para escribir variantes a medida |
| **`recover` dentro de cada goroutine hija** | un pánico en una goroutine **no** lo recoge nadie más y mata el proceso: conviértelo en error |
| **Escritura por índice** (`res[i] = …`) | cada goroutine escribe en una posición propia: sin cerrojo |

### Pruebas

- `testing/synctest`: el tiempo virtual hace los tests de cancelación **exactos** (p. ej. «vuelve a los 10 ms») y detecta goroutines que sobreviven al bloque.
- `goleak.VerifyTestMain`: confirma que ningún test deja goroutines vivas.

## Qué hace el programa

| Archivo | Contenido |
|--|--|
| `api.go` | pares **mal / bien**: `RelojMal` frente a `Reloj.Run(ctx)`, `Servicio` (`Iniciar`/`Close`), `DuplicarMal` frente a `Duplicar(ctx)` y `DuplicarIter`, `DescargarMal` frente a `DescargarTodas` (errgroup) y `DescargarWG` |
| `main.go` | cuenta goroutines antes/después de cada uso |
| `api_test.go` | fugas (`NumGoroutine`), cancelación exacta con `synctest`, límite de concurrencia, pánico→error, `goleak` |

```bash
go run .
go test -race -v .
```

## Resultados de referencia

```
== 1) ¿quién lanza la goroutine? ==
  100 × NuevoRelojMal()      → +100 goroutines (sin forma de pararlas)
  100 × go Reloj.Run(ctx)    → +100 … tras cancelar(ctx): +0

== 2) API de flujo con un consumidor que abandona a mitad ==
  DuplicarMal ×100 (lee 1 de 3)        → +100 goroutines bloqueadas para siempre
  Duplicar(ctx) ×100                   → +0
  DuplicarIter ×100 (break)            → +0 (iterador síncrono)

== 3) 1 URL falla a los 20 ms, las otras 9 tardan 500 ms ==
  DescargarTodas : vuelve a los 20 ms con error "mala: 503"; goroutines vivas tras volver: +0
  DescargarMal   : vuelve a los 21 ms con error "503";       goroutines vivas tras volver: +9
```

## Conclusiones

- **El diseño con constructor que lanza goroutines fuga una por instancia** (+100 con 100 instancias); con `Run(ctx)` el llamante las lanza **y** las para con una sola cancelación (+0 tras `cancel`).
- **Un channel sin buffer que nadie lee fuga** una goroutine por llamada (+100). La solución con `ctx` y la del iterador no dejan ninguna; el **iterador** es la más simple porque no hay concurrencia que gestionar.
- **A simple vista, `DescargarMal` y `DescargarTodas` tardan lo mismo en devolver el error (≈ 20 ms)**, pero la mala deja **9 goroutines vivas** descargando 480 ms más —y escribiendo en un slice que el llamante ya está usando— mientras la estructurada **las canceló y las esperó**: al volver, no queda nada.
- `SetLimit(3)` con 10 tareas: el máximo observado de tareas simultáneas es **exactamente 3** (test determinista con `synctest`).
- Un pánico en una goroutine hija sin `recover` mata todo el proceso; con `recuperar` se convierte en un error normal del `errgroup`.
- Pregunta de diseño: **¿tu función debe devolver un channel, un iterador o un slice?** Si el consumidor lo recorre secuencialmente y no necesitas paralelismo, un iterador; si hay productor concurrente, un channel + `ctx`; si cabe en memoria y no importa la latencia, un slice.

## Ejercicios propuestos

1. Añade a `Servicio` un `Reiniciar` con *backoff* exponencial cuando la tarea termina con error (supervisor).
2. Escribe `Mapear(ctx, entrada <-chan A, n int, f func(A) B) <-chan B` con `n` workers que respeta `ctx` y conserva el orden.
3. Convierte `Duplicar` en `iter.Seq` con paralelismo interno usando `iter.Pull` y razona sobre las fugas si el consumidor no llama a `stop`.
4. Documenta cada API con un comentario *«Goroutines: …»* que diga quién las lanza, cuándo terminan y qué hacer para pararlas.
