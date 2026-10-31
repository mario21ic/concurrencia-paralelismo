# Ejercicio 7: worker con apagado ordenado (Ctrl+C)

## Enunciado

Un servicio con un productor y N workers debe detenerse limpiamente al recibir `Ctrl+C` (SIGINT) o `kill` (SIGTERM):

1. Dejar de aceptar trabajo nuevo.
2. Terminar (o abortar) lo que está en curso.
3. Esperar a los workers con un **tiempo de gracia** máximo.
4. Si llega una segunda señal, terminar de inmediato.

## Teoría

### `signal.NotifyContext`

```go
ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
defer stop()
```

Devuelve un contexto que se cancela al llegar la primera señal indicada. `stop()` libera el registro y **restaura el comportamiento por defecto**, de modo que una segunda `Ctrl+C` mata el proceso aunque el apagado ordenado esté colgado.

### Apagado ordenado (graceful shutdown)

```
señal ─► ctx cancelado
          ├─ productor: deja de producir y cierra `jobs`
          ├─ workers: terminan el trabajo actual y salen del `range`
          └─ main: espera wg.Wait() con tiempo de gracia
```

Decisiones de diseño:

- **El productor es dueño del cierre de `jobs`** (módulo 3). Los workers no necesitan mirar `ctx` para parar: al cerrarse `jobs` terminan su `range`.
- Dos políticas de trabajo en curso:
  - *Terminar* (por defecto): el trabajo en curso se completa, no se pierde nada.
  - *Abortar* (`-abortar`): el trabajo revisa `ctx` entre pasos y se interrumpe.
- **Tiempo de gracia** (`-gracia`): `select` entre `hecho` y `time.After`. En Kubernetes coincide con `terminationGracePeriodSeconds` (30 s por defecto, y luego SIGKILL).
- El código de salida es 1 si se agota la gracia, para que el orquestador lo vea.
- Si el productor y los workers hacen `select` con `ctx.Done()` en cada envío, nada se queda bloqueado esperando a un receptor que ya no existe.

## Cómo ejecutarlo

```bash
go run . -workers 3                   # interactivo: pulsa Ctrl+C
go run . -workers 3 -abortar          # interrumpe el trabajo en curso
go run . -duracion 1s                 # se apaga sola (WithTimeout sobre el contexto de señales)
go run . -workers 1 -gracia 50ms      # fuerza que se agote la gracia
```

Probar con señales desde otra terminal:

```bash
kill -INT  <pid>      # igual que Ctrl+C
kill -TERM <pid>
```

> Si lo lanzas con `&` desde un script no interactivo, el shell deja SIGINT **ignorado** para procesos en segundo plano y Go respeta esa disposición; usa `kill -TERM` en esos casos.

## Resultados de referencia

Terminar el trabajo en curso (SIGINT):

```
pid=43859, 2 workers. Pulsa Ctrl+C para apagar.
worker 2 procesando trabajo 1
worker 1 procesando trabajo 2
worker 2 procesando trabajo 3
worker 1 procesando trabajo 4
productor: no se aceptan más trabajos

apagando (interrupt signal received)... un segundo Ctrl+C fuerza la salida
worker 2 terminó
worker 1 terminó
apagado ordenado completado
resumen: 4 completados, 0 abortados
```

Abortar (SIGTERM con `-abortar`):

```
apagando (terminated signal received)...
  trabajo 5 abortado en el paso 2
...
resumen: 4 completados, 1 abortados
```

Gracia agotada (`exit=1`):

```
tiempo de gracia agotado: salida forzada
```

Segunda señal durante el apagado: el proceso muere de inmediato (`exit=143` con SIGTERM).

## Conclusiones

- `context.Cause(ctx)` indica el origen del apagado (`interrupt signal received`, `terminated signal received`, `context deadline exceeded`).
- Con `-duracion` el mismo código sirve para pruebas automatizadas sin señales.
- El patrón es el esqueleto de cualquier servicio: contexto raíz con señales → todos los componentes lo reciben → `wg.Wait()` con límite.

## Ejercicios propuestos

1. Añade un `http.Server` y apágalo con `server.Shutdown(ctxConGracia)`.
2. Sustituye `wg` + gracia por `errgroup` (módulo 10).
3. Persiste los trabajos no procesados (los que quedaron sin enviar) antes de salir.
4. Escribe un test que lance el worker con `-duracion` y verifique `completados + abortados`.
