# Ejercicio 6: consultar N URLs en paralelo

## Enunciado

Dada una lista de URLs, obtener el estado HTTP y el tamaño de cada una de forma concurrente, y compararlo con la versión secuencial.

Para no depender de internet, el programa levanta un servidor local `httptest` que responde con latencias de 50 a 250 ms y falla a propósito (HTTP 500) para algunas rutas.

## Teoría y diseño

- Esta carga es **I/O-bound**: casi todo el tiempo es espera. Una goroutine por URL es adecuada y barata.
- Cada goroutine escribe en **su propia posición** `out[i]` de un slice preasignado. No hay race porque:
  1. los índices son distintos, y
  2. `wg.Wait()` establece happens-before antes de leer el slice.
  Así no se necesita mutex ni channel.
- Los errores (red o HTTP) se guardan **en el resultado**, no se pierden ni abortan al resto.
- **Limitaciones** (que se resuelven en módulos siguientes): lanza todas a la vez sin límite (módulo 8: worker pool/semáforo), no hay timeout ni cancelación (módulo 5: `context`), no hay reintentos ni `errgroup` (módulo 10).

## Cómo ejecutarlo

```bash
go run .
go run -race .
```

## Resultado de referencia

```
URLs: 20, fallos HTTP: 2
secuencial: 3.005s
concurrente: 245ms (12.3x)
  http://127.0.0.1:63589/1         estado=200 bytes=11 en 89ms
  http://127.0.0.1:63589/2         estado=200 bytes=11 en 126ms
```

El tiempo concurrente ≈ la latencia de la URL más lenta, no la suma.

## Ejercicios propuestos

1. Sube a 10 000 URLs: ¿qué pasa con los descriptores de archivo (`ulimit -n`)? Pista: necesitarás limitar la concurrencia.
2. Añade un `http.Client{Timeout: 150 * time.Millisecond}` y cuenta cuántas expiran.
3. Reescribe usando un channel de resultados en lugar del slice indexado. ¿Se pierde el orden original?
