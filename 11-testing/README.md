# Módulo 11. Testing de código concurrente

**Objetivo:** escribir pruebas **rápidas, deterministas y que detecten** los fallos propios de la concurrencia: carreras, orden dependiente, dependencia del tiempo real, fugas de goroutines y errores de lógica que solo aparecen en ciertas interleavings.

## Ejercicios

| # | Carpeta | Tema |
|---|---------|------|
| 1 | [01-flags-y-race](01-flags-y-race) | `-race`, `-count`, `-shuffle`, `-cpu`: reproducir fallos intermitentes |
| 2 | [02-sin-sleep](02-sin-sleep) | Tests deterministas sin `time.Sleep`: inyectar el tiempo y sincronizar con eventos |
| 3 | [03-synctest-basico](03-synctest-basico) | `testing/synctest`: burbuja, reloj virtual, `Wait` |
| 4 | [04-synctest-aplicado](04-synctest-aplicado) | Reintentos, debounce y caché con TTL probados en horas virtuales |
| 5 | [05-fugas](05-fugas) | Detectar goroutine leaks: `goleak`, `synctest`, ayudante propio, perfil `goroutineleak` |
| 6 | [06-estres-y-fuzz](06-estres-y-fuzz) | Pruebas de estrés y fuzzing contra un modelo |

```bash
cd 11-testing/03-synctest-basico && go test -race -v .
```

### Etiquetas de compilación (tests que fallan a propósito)

Para poder **ver** los defectos sin romper `go test ./...`, los tests que fallan a propósito están tras etiquetas:

| Etiqueta | Dónde | Qué activa |
|--|--|--|
| `bugs` | ejercicios 1, 5, 6 | tests que fallan por un defecto real |
| `sleep` | ejercicio 2 | el test con `Sleep` (lento y frágil) |
| `bugsynctest` | ejercicio 5 | el test que provoca el pánico de `synctest` |

```bash
go test -tags bugs -race .      # ver los defectos
go test ./...                   # todo en verde
```

## Resumen teórico

### Por qué es difícil

Un test concurrente que **pasa** no prueba nada: el fallo depende de la interleaving, de la carga, del número de CPUs y de la suerte. Hay que combinar herramientas que atacan cada tipo de fallo:

| Tipo de fallo | Herramienta |
|--|--|
| Data race | `go test -race` |
| Orden de ejecución / estado global | `-shuffle=on`, `t.Cleanup` |
| Fallo que depende del paralelismo real | `-cpu 1,2,8`, `-count=N` |
| Dependencia del tiempo real (lento, frágil) | tiempo inyectado, **`synctest`** |
| Goroutine leak | `goleak`, `synctest`, perfil `goroutineleak` |
| Bug lógico en ciertas secuencias | **fuzzing** contra un modelo |
| Pérdida/duplicado bajo carga | estrés + invariantes de conservación |

### Opciones de `go test`

| Flag | Efecto |
|--|--|
| `-race` | detector de carreras (≈ 5–10× más CPU y memoria) |
| `-count=N` | repite N veces (y desactiva la caché de resultados) |
| `-shuffle=on` / `=<semilla>` | aleatoriza el orden de tests; imprime la semilla para reproducir |
| `-cpu 1,2,8` | ejecuta con distintos `GOMAXPROCS` |
| `-parallel N` | límite de tests con `t.Parallel()` |
| `-timeout 60s` | aborta si se cuelga (¡siempre en tests concurrentes!) |
| `-failfast` | parar en el primer fallo |
| `-run regex` / `-v` | filtrar / detallar |
| `-fuzz F -fuzztime 10s` | fuzzing |
| `-tags x` | activa archivos con `//go:build x` |

### El detector de carreras (`-race`)

- Detecta **accesos concurrentes sin orden** (*happens-before*) a la misma memoria, **cuando ocurren** en la ejecución. No encuentra los que no se ejercitan.
- Cero falsos positivos reales (si lo dice, es una carrera); puede tener falsos negativos.
- No detecta errores de **atomicidad lógica** (check-then-act con locks correctos) ni deadlocks.
- Es probabilístico: puede aparecer solo a veces (en el ejercicio 3 apareció 1 de 3 ejecuciones). Úsalo con `-count`.

### `testing/synctest` (estable desde Go 1.25)

`synctest.Test(t, func(t *testing.T) { ... })` ejecuta el test en una **burbuja**:
- Un **reloj virtual** (empieza el 2000-01-01 UTC) que **avanza solo cuando todas las goroutines de la burbuja están bloqueadas de forma durable**, saltando al siguiente evento. `time.Sleep(24 * time.Hour)` tarda microsegundos.
- `synctest.Wait()` bloquea hasta que las demás goroutines estén bloqueadas: permite afirmar sobre estados intermedios **sin dormir**.
- Al terminar, espera a que **todas** las goroutines salgan; si alguna sigue bloqueada, **`Test` entra en pánico** (`deadlock: main bubble goroutine has exited but blocked goroutines remain`). Es un detector de fugas gratis.
- `t.Context()` se cancela justo antes de `Cleanup`: úsalo para terminar goroutines de fondo.
- Bloqueos **durables**: operaciones sobre channels creados dentro, `select` solo con esos channels, `time.Sleep`, `sync.Cond.Wait`, `sync.WaitGroup.Wait` (con `Add` dentro). **No** durables: `sync.Mutex`, E/S de red, llamadas al sistema, channels creados fuera.
- Prohibido dentro: `t.Run`, `t.Parallel`, `t.Deadline`. No usar red real (usa `net.Pipe` o dobles).
- **No sustituye la sincronización de memoria**: `-race` sigue aplicando (ejercicio 3).

### Regla de oro: nunca sincronices con `time.Sleep`

`Sleep` en un test significa "espero que ya haya pasado lo que esperaba". Es lento (se suma) y frágil (falla bajo carga). Alternativas, de mejor a peor:

1. **`synctest`** si el código depende del tiempo.
2. **Inyectar** el tiempo (`tick <-chan time.Time`, `now func() time.Time`) y controlarlo desde el test.
3. **Sincronizar con eventos**: channels, `WaitGroup`, `Cond`, callbacks que notifican (`volcar` envía por un channel).
4. **Sondeo con plazo** (`Eventualmente`): último recurso.

Los plazos (`select` con `time.After(2s)`) son legítimos solo como **red de seguridad** para que un bug *falle* en lugar de colgar.

### Diseñar para poder probar

- Recibe el **reloj**, el **ticker** y los **callbacks** como dependencias.
- Devuelve un channel o expón un método que **espere** al fin (`Cerrar()` que haga `<-hecho`).
- Evita estado global; si no, restáuralo con `t.Cleanup`.
- Toda goroutine con salida clara (`ctx`) ⇒ fácil de probar que termina.

## Preguntas de repaso

1. ¿Por qué un test concurrente que pasa no demuestra que el código sea correcto?
2. ¿Qué detecta `-race` y qué no? Da un ejemplo de cada caso.
3. ¿Por qué `-cpu 1` puede esconder un bug de carrera? ¿Y por qué puede *revelar* otro (ejercicio 6)?
4. ¿Cómo avanza el reloj de una burbuja de `synctest`?
5. ¿Qué es un bloqueo "durable" y por qué un `sync.Mutex` no lo es?
6. ¿Por qué `synctest.Test` entra en pánico si queda una goroutine bloqueada?
7. ¿Por qué `time.Sleep` en un test es mala idea? Nombra tres alternativas.
8. ¿Qué tipos de bug encuentra el fuzzing frente al test de estrés?
9. ¿Cómo reproduces un fallo de `-shuffle`?

## Lecturas

- [Testing concurrent code with testing/synctest](https://go.dev/blog/synctest) (blog de Go)
- Documentación de [`testing/synctest`](https://pkg.go.dev/testing/synctest)
- [Data Race Detector](https://go.dev/doc/articles/race_detector)
- [Go Fuzzing](https://go.dev/doc/security/fuzz/)
- [`go.uber.org/goleak`](https://pkg.go.dev/go.uber.org/goleak)
- Notas de Go 1.26 sobre el perfil experimental `goroutineleak`
