# Ejercicio 3: perfil de goroutines — qué hacen y dónde se acumulan

## Teoría

El perfil `goroutine` es una **foto instantánea de todas las goroutines vivas** con su pila. Es la herramienta principal para dos preguntas:

1. **¿Por qué está colgado el programa?** → mirar en qué estado y dónde está cada goroutine.
2. **¿Hay una fuga?** → ver si un mismo grupo de pilas crece con el tiempo.

### Formatos

| Petición | Salida |
|--|--|
| `/debug/pprof/goroutine` (binario) | para `go tool pprof -top/-http` |
| `?debug=1` | **agrupado por pila**: `N @ 0x… 0x…` seguido de las funciones. El grupo con mayor `N` es el sospechoso |
| `?debug=2` | **una cabecera por goroutine**: `goroutine 7 [chan receive, 2 minutes]:` con la pila completa, el **estado** y el **tiempo bloqueada** |

También `kill -QUIT <pid>` (SIGQUIT) vuelca todas las pilas a stderr y termina el proceso; y `GOTRACEBACK=all` en un pánico.

### Estados de espera que verás

| Estado entre corchetes | Significa |
|--|--|
| `chan receive` / `chan send` | bloqueada en un channel |
| `select` / `select (no cases)` | en un `select` (el segundo: `select{}`, bloqueada para siempre) |
| `sleep` | `time.Sleep` o un timer |
| `sync.Mutex.Lock` / `sync.RWMutex.RLock` | esperando un lock |
| `sync.WaitGroup.Wait`, `sync.Cond.Wait` | esperando a otras goroutines |
| `IO wait` | esperando E/S de red (el *netpoller*) |
| `syscall` | dentro de una llamada al sistema |
| `runnable` / `running` | lista para ejecutar / ejecutando |

El sufijo `, N minutes` indica **cuánto** lleva en ese estado: una goroutine bloqueada hace 40 minutos en `chan send` es casi seguro una fuga.

### Método para encontrar una fuga

1. Tomar el perfil `debug=1` **dos veces** con tiempo entre medias (o antes/después de una carga).
2. Comparar el recuento total y los grupos: **el que crece** es el culpable.
3. Mirar la **primera función propia** de su pila: ahí está la línea que bloquea.
4. Entender *por qué nadie la despierta* (nadie recibe, nadie cierra, falta cancelación) y arreglarlo (buffer, `select` con `ctx.Done()`, `close`).

`AgruparPilas` automatiza el paso 2-3 a partir de la salida de `debug=1`.

## Qué hace el programa

**Parte 1:** deja goroutines bloqueadas por motivos distintos (channel, `select`, `Sleep`, mutex, `Cond`, socket TCP real) y las cuenta por estado con `debug=2`.

**Parte 2:** un handler HTTP que fuga una goroutine por petición (envía a un channel sin buffer tras vencer el plazo del handler). Se expone `net/http/pprof`, se hacen 200 peticiones y se analiza `/debug/pprof/goroutine?debug=1`.

```bash
go run .               # con fuga
go run . -arreglado    # con channel de buffer 1: sin fuga
go test -race .
```

## Resultados de referencia

```
== Parte 1: goroutines por estado (perfil con debug=2) ==
  sync.Mutex.Lock        4
  chan receive           3
  chan send              2
  sleep                  2
  sync.Cond.Wait         1
  IO wait                1
  running                1      <- la que está generando el propio perfil
  select                 1

== Parte 2: tras 200 peticiones (arreglado=false) ==
  goroutines: antes=10 después=210 (+200)
  grupos de pilas más grandes:
     200 × main.main.func2.1 (…/main.go:122)        <- el culpable, con la línea exacta
       2 × main.goroutinesEnEstadosDistintos.func2 (…)
       2 × time.Sleep (…)

== Parte 2: (arreglado=true) ==
  goroutines: antes=10 después=10 (+0)
```

## Conclusiones

- Cada tipo de bloqueo tiene su estado propio: leer el estado ya orienta el diagnóstico (¿channel? ¿mutex? ¿red?).
- La fuga se identifica por **un grupo de 200** goroutines con la misma pila, y la línea 122 es exactamente `resultado <- "ok"`.
- La corrección (buffer de 1) deja el recuento en **+0**.
- Compara con el módulo 11: ahí se detecta la fuga **en un test**; aquí **en un servicio en ejecución**, sin parar nada.

## Tests

`AgruparPilas` (agrupación, orden y extracción de función y línea) y `ContarPorEstado` (con y sin tiempo de espera en la cabecera).

## Seguridad

`net/http/pprof` expone pilas, memoria y permite iniciar perfiles de CPU: **nunca lo publiques en un puerto accesible desde internet**. Úsalo en un puerto interno (`127.0.0.1`), tras autenticación o en un `ServeMux` propio.

## Ejercicios propuestos

1. Usa `go tool pprof -http=:8081 http://…/debug/pprof/goroutine` para ver el grafo.
2. Toma dos fotos con 30 s de diferencia y compara con `go tool pprof -base antes.pb.gz despues.pb.gz`.
3. Envía `kill -QUIT` a un programa de ejemplo y lee el volcado.
4. Añade el recuento de goroutines como métrica (`runtime/metrics`, ejercicio 6) y una alerta si crece de forma sostenida.
