# Ejercicio 4: `time.Ticker`

## Teoría

Un `Ticker` envía la hora actual por `t.C` cada periodo `d`, de forma repetida.

- **Siempre `Stop()`** cuando termines. Desde Go 1.23 un ticker sin referencias se recolecta, pero parar explícitamente sigue siendo la buena práctica y deja la intención clara.
- El channel tiene capacidad efectiva 1: si el consumidor es **más lento** que el periodo, los ticks sobrantes se **descartan** (no se acumulan ni se recuperan). Un ticker da *frecuencia máxima*, no *garantía de N ejecuciones*.
- `Ticker.Reset(d)` cambia el periodo.
- Ticker vs `Sleep` en bucle: `Sleep(d)` en un bucle deriva (cada vuelta suma la duración del trabajo); un ticker mantiene el ritmo respecto al reloj.
- `time.Tick` no tiene `Stop`; úsalo solo en programas cortos o `main`.

### Heartbeat / trabajos periódicos

```go
tk := time.NewTicker(d)
defer tk.Stop()
for {
    select {
    case <-tk.C:   // trabajo periódico
    case <-done:   // salida limpia
        return
    }
}
```

La goroutine siempre debe tener el caso `done`/`ctx.Done()`; sin él es una fuga.

## Qué hace el programa

1. 4 ticks de 100 ms, con la hora de cada uno.
2. Ticker de 10 ms con trabajo de 35 ms durante 300 ms: cuántos ticks llegan.
3. Heartbeat de 50 ms que se detiene cerrando `done`.

```bash
go run .
```

## Resultado de referencia

```
tick 1 a los 100ms ... tick 4 a los 400ms

ticks recibidos: 10 (teóricos: 30)

latido / latido / latido
heartbeat detenido
```

## Conclusiones

- El ticker mantiene el ritmo (100, 200, 300, 400 ms).
- Con un consumidor lento se reciben ~10 de 30 ticks: el resto se pierde. Si cada tick debe procesarse, necesitas una cola o un diseño distinto.
- Siempre debe haber una vía de salida y un `Stop`.

## Ejercicios propuestos

1. Haz que el trabajo de 35 ms corra en una goroutine aparte para no perder ticks. ¿Qué riesgo aparece (solapamiento)?
2. Compara un bucle con `time.Sleep(100ms)` y uno con `Ticker` cuando el trabajo tarda 30 ms; mide la deriva tras 50 iteraciones.
3. Implementa un limitador simple de 5 operaciones/segundo con `Ticker`.
