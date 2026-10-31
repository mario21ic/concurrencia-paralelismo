# Ejercicio 4: `runtime.LockOSThread`

## Teoría

Normalmente una goroutine puede ejecutarse en **cualquier** hilo del SO y migrar entre ellos. `runtime.LockOSThread()` establece una relación **1 a 1**: esa goroutine solo se ejecuta en ese hilo, y ese hilo **solo ejecuta esa goroutine** hasta que se llame a `UnlockOSThread()` (las llamadas se acumulan: hace falta una `Unlock` por cada `Lock`).

### Cuándo es necesario

El estado vive en el **hilo**, no en la goroutine:

| Caso | Por qué |
|--|--|
| **GUI / OpenGL / Metal / Cocoa** | exigen que ciertas llamadas se hagan siempre desde el **hilo principal** o desde el hilo que creó el contexto |
| **Namespaces de Linux** (`setns`, `unshare`), `setuid` por hilo | el cambio afecta al hilo |
| **Bibliotecas C con estado en TLS** (almacenamiento local del hilo) | la segunda llamada debe ver el mismo hilo |
| **Afinidad de CPU, prioridad del hilo** | se aplican al hilo del SO |

### El hilo principal

`main` empieza en el hilo principal del proceso. Receta estándar:

1. En `init()`: `runtime.LockOSThread()` (las funciones `init` se ejecutan en la goroutine principal, fijándola al hilo principal).
2. En `main()`: un **bucle de eventos** que recibe trabajos por un channel y los ejecuta.
3. Las demás goroutines envían sus llamadas a ese bucle (`EnHiloPrincipal(f)`).

### Reglas y trampas

- **Una goroutine que termina sin soltar el hilo lo destruye** (el runtime no lo reutiliza porque podría estar contaminado). En un bucle, eso es una fábrica de hilos. Haz `defer runtime.UnlockOSThread()`.
- **Las goroutines que crea una goroutine fijada NO heredan el hilo.**
- **Es caro si hay mucha comunicación:** cada traspaso entre goroutines fijadas necesita un cambio de contexto del **SO** (futex/semáforo), no del runtime.

## Qué hace el programa

| Archivo | Contenido |
|--|--|
| `main.go` | los 4 experimentos |
| `hilo_principal.go` | `init` + `EnHiloPrincipal(f)` + `BucleDelHiloPrincipal` |
| `tid_darwin.go`, `tid_linux.go`, `tid_otros.go` | identificador del hilo del SO por plataforma (`pthread_threadid_np`, `gettid`) |
| `main_test.go` | una goroutine fijada ve siempre el mismo hilo; las llamadas llegan al hilo principal |

```bash
go run .
go test -race -v .
```

## Resultados de referencia (Apple M1, macOS, Go 1.26.5)

```
1) ¿en cuántos hilos del SO se ejecuta una goroutine que se duerme 200 veces?
   sin LockOSThread: 4 hilos distintos    con LockOSThread: 1 hilo

2) el patrón del hilo principal (GUI/OpenGL): 10 de 10 llamadas se ejecutaron en el hilo principal

3) ping-pong entre dos goroutines (200 000 vueltas)
   normales:        280 ns por vuelta
   ambas fijadas:   27.22 µs por vuelta  (96× más lento)

4) goroutines que se fijan y terminan SIN soltar el hilo: 200 hilos del SO distintos
   las que se fijan y SUELTAN el hilo:                     1 hilo (se reutilizan)
```

## Conclusiones

- **Sin `LockOSThread` la goroutine migra** (4 hilos distintos en 200 despertares); con él, siempre el mismo.
- **El patrón del hilo principal funciona:** las 10 llamadas pasaron por el hilo en que corrió `init`.
- **Comunicar goroutines fijadas es 96× más lento** (280 ns → 27 µs): el traspaso pasa del planificador de Go al del SO. Fija **solo** lo imprescindible y mantén el trabajo sin fijar en goroutines normales.
- **Olvidar `Unlock` crea un hilo por goroutine** (200 hilos frente a 1). En un servidor con miles de peticiones, esto agota `SetMaxThreads`.

## Ejercicios propuestos

1. Haz que `EnHiloPrincipal` devuelva un valor y propague pánicos al llamante.
2. En Linux, usa `unix.Setns`/`Unshare` en una goroutine fijada y comprueba que otras goroutines no ven el cambio.
3. Mide el coste del ping-pong con **una** goroutine fijada y una normal.
4. Escribe un test que falle si alguna ruta del código termina una goroutine fijada sin `Unlock`.
