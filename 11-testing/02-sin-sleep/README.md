# Ejercicio 2: tests deterministas sin `time.Sleep`

## Teoría

Un `Acumulador` agrupa elementos y los vuelca **por tamaño**, **por tiempo** o **al cerrarse**. El tiempo plantea la pregunta: ¿cómo pruebo "vuelca cada 50 ms" sin esperar 50 ms (y sin que falle si la máquina va lenta)?

### La mala versión (`sleep_test.go`, `-tags sleep`)

```go
tk := time.NewTicker(50 * time.Millisecond)
a := Nuevo(100, tk.C, volcar)
a.Agregar(1); a.Agregar(2)
time.Sleep(120 * time.Millisecond)   // "espero que ya haya vuelto a pasar el tick"
// ... comprobar
```

Problemas: **lenta** (120 ms por ejecución, suman), **frágil** (si el ticker se retrasa más de 70 ms, falla) y **no determinista** (el ticker real no es controlable).

### La buena versión: inyectar el tiempo y sincronizar con eventos

1. **El tiempo es una dependencia.** `Nuevo` recibe `tick <-chan time.Time`. En producción: `time.NewTicker(d).C`. En el test: un `chan time.Time` propio; `tick <- time.Time{}` significa "pasó el intervalo". Cero esperas.
2. **Los resultados llegan por un channel.** El callback `volcar` envía al channel `lotes`; el test lo lee. Es una **sincronización real**: el test avanza cuando ocurre el evento.
3. **Orden garantizado por rendezvous.** `Agregar` envía por un channel sin buffer, así que al retornar el bucle ya recibió el elemento; el siguiente `tick <- ...` solo se acepta cuando el bucle vuelve a su `select`, es decir, después de procesar el elemento. No hace falta dormir.
4. **`Cerrar()` espera a la goroutine** (`<-hecho`): al volver, el volcado final ya ocurrió.
5. **Plazo de seguridad**: `recibir` usa `select` con `time.After(2s)` solo para que un bug *falle* en vez de colgar; nunca para sincronizar.
6. **`noRecibir`** (con `default`) comprueba que *no* hay lote *ahora*: aserción negativa sin esperar.

### Último recurso: `Eventualmente`

Si no hay forma de inyectar nada, `Eventualmente(t, plazo, cond, msg)` sondea la condición. Sigue usando tiempo real (plazo y paso de 1 ms), pero al menos retorna en cuanto se cumple en lugar de esperar un tiempo fijo.

## Cómo ejecutarlo

```bash
go test -race -v .                     # los 5 tests deterministas
go test -race -count=200 .             # 200 repeticiones
go test -tags sleep -run ConSleep -count=20 .     # la versión con Sleep
```

## Resultados de referencia

```
5 tests deterministas × 200 repeticiones, con -race:        1.2 s  (casi todo es compilación/arranque)
1 test con Sleep(120ms) × 20 repeticiones:                  2.6 s  (≈ 129 ms por repetición)
```

Cada test determinista tarda 0.00 s. El de `Sleep` cuesta más de 100 ms **por ejecución**, y esa cifra crece linealmente con cada test que duerma.

## Conclusiones

- Inyectar el tiempo elimina la espera y la fragilidad a la vez.
- Los channels son el mecanismo natural de sincronización en un test: el test **espera un evento**, no un tiempo.
- La mejor señal de calidad: poder ejecutar el test miles de veces con `-race -count=1000`.
- Si el código no permite inyectar el tiempo, es una señal para **rediseñarlo** (o usar `synctest`, ejercicio 3).

## Ejercicios propuestos

1. Cambia `tick` por un `Reloj` con método `Ahora()` y un `Ticker()` falso controlable.
2. Añade una versión con `context.Context` para cancelar y pruébala sin dormir.
3. Introduce un bug (volcar dos veces el último lote) y comprueba que los tests lo detectan.
4. Convierte `TestVuelcaPorTiempo_ConSleep` con `synctest` (ejercicio 3) y compara el código.
