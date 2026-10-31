# Ejercicio 3: `testing/synctest`

## Teoría

`testing/synctest` (estable desde Go 1.25) permite probar código dependiente del tiempo **sin esperar** y de forma **determinista**.

```go
synctest.Test(t, func(t *testing.T) {
    // aquí dentro: reloj virtual
    time.Sleep(24 * time.Hour) // retorna al instante
})
```

### El modelo

- **Burbuja:** `Test` ejecuta `f` en una burbuja. Las goroutines que lance `f` pertenecen a ella.
- **Reloj virtual:** `time.Now()` empieza el **2000-01-01 00:00:00 UTC**. Cada burbuja tiene el suyo.
- **El tiempo solo avanza cuando todas las goroutines de la burbuja están bloqueadas de forma durable**; entonces salta al **siguiente evento** (un timer, un `Sleep`…). Si hay alguna goroutine corriendo, no avanza.
- **`Wait()`** bloquea hasta que las *demás* goroutines estén durablemente bloqueadas: "deja que todo lo que pueda ocurrir, ocurra".
- Al terminar `f`, `Test` espera a que todas las goroutines salgan. Si quedan bloqueadas → pánico (ver ejercicio 5).
- El tiempo **deja de avanzar cuando termina la goroutine raíz** de la burbuja.

### Qué bloquea de forma durable

| Durable (el reloj puede avanzar) | No durable (podría desbloquearlo algo externo) |
|--|--|
| enviar/recibir en un channel **creado en la burbuja** | `sync.Mutex` / `RWMutex` |
| `select` con solo channels de la burbuja | E/S de red o de archivos |
| `time.Sleep` | llamadas al sistema |
| `sync.Cond.Wait` | channels creados **fuera** |
| `sync.WaitGroup.Wait` (con `Add` dentro) | |

Si una goroutine está bloqueada en algo **no durable**, `Wait` no la considera "quieta" y el reloj no avanza por su culpa.

### Aislamiento

- Un channel, `Timer` o `Ticker` creado dentro está asociado a la burbuja: usarlo desde fuera provoca pánico.
- Un `WaitGroup` queda ligado a la burbuja en su primer `Add`/`Go`.
- Dentro **no** se puede llamar a `t.Run`, `t.Parallel` ni `t.Deadline`.
- Usa `t.Context()` (se cancela justo antes de `Cleanup`) para detener goroutines de fondo.
- No uses red real; sustituye por `net.Pipe` o dobles de prueba.

### Lo que NO hace

**`synctest` hace determinista el *tiempo*, no la *memoria*.** Si dos goroutines acceden a una variable sin sincronización, `-race` sigue avisando (ver "una carrera real" abajo).

## Qué contiene

Solo tests (`reloj_test.go`); no hay código de producción.

| Test | Muestra |
|--|--|
| `RelojVirtual` | `Sleep(24h)` instantáneo; hora inicial 2000-01-01; `Since` exacto |
| `OrdenDeDespertar` | 3 sleeps de 3 s, 1 s y 2 s despiertan siempre en orden 1, 2, 3 |
| `Wait` / `EstadoIntermedio` | afirmar "todavía no ha ocurrido" y "ya ocurrió", sin dormir |
| `Ticker` | un ticker de 1 s produce exactamente 5 ticks en 5.5 s |
| `ContextConPlazo` | `WithTimeout(1m)` vence en exactamente 1 minuto virtual |
| `AfterFunc` | `time.AfterFunc` y `context.AfterFunc`, con precisión de **1 ns** |
| `SelectConTimeout` | un `time.After(time.Hour)` no cuesta una hora |
| `WaitGroupYCond` | `WaitGroup.Wait` y `Cond.Wait` permiten avanzar el reloj |

```bash
go test -race -v .
go test -race -count=100 .
```

## Resultado de referencia

```
--- PASS: TestRelojVirtual (0.00s)
--- PASS: TestOrdenDeDespertar (0.00s)
--- PASS: TestWait (0.00s)
--- PASS: TestEstadoIntermedio (0.00s)
--- PASS: TestTicker (0.00s)
--- PASS: TestContextConPlazo (0.00s)
--- PASS: TestAfterFunc (0.00s)
--- PASS: TestSelectConTimeout (0.00s)
--- PASS: TestWaitGroupYCond (0.00s)
ok   1.2s   (100 repeticiones con -race)
```

Todos suman más de **26 horas** de tiempo virtual y tardan 0.00 s.

## Una carrera real (aprendida durante el desarrollo)

La primera versión de `TestAfterFunc` usaba un `bool` plano:

```go
llamado := false
time.AfterFunc(10*time.Minute, func() { llamado = true })   // escribe más tarde
time.Sleep(10*time.Minute - time.Nanosecond)
synctest.Wait()
if llamado { ... }        // el test LEE antes de que el callback ESCRIBA
```

Con `-race`, falló **1 de cada 3 ejecuciones**: el detector avisó de una carrera de datos. La lectura (en el test) y la escritura (en la goroutine del callback) **no tienen ningún orden definido** por el modelo de memoria, aunque el reloj virtual haga que en la práctica ocurran siempre en el mismo orden. Solución: `atomic.Bool`.

Lección: `synctest` elimina la no determinación del **tiempo**; la **sincronización de memoria** sigue siendo tu responsabilidad. Y `-race` sigue siendo imprescindible (y probabilístico: hay que repetir).

## Conclusiones

- Pruebas que "esperan" minutos, horas o días corren en microsegundos.
- Los resultados son exactos (`== 300ms`), no aproximados (`>= 290ms`).
- `Wait` sustituye a los `Sleep` de "dale tiempo a que pase".
- Es una herramienta de **pruebas**: no usar en código de producción.

## Ejercicios propuestos

1. Prueba un `time.Ticker` que se `Reset` a mitad.
2. Comprueba qué pasa si una goroutine de la burbuja se bloquea en un `sync.Mutex` y llamas a `Wait`.
3. Intenta usar un channel creado fuera de la burbuja y lee el pánico.
4. Convierte el test `TestVuelcaPorTiempo_ConSleep` del ejercicio 2 a `synctest`.
