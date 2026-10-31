# Ejercicio 5: *Flight Recorder* (`runtime/trace`, Go 1.25+)

## Teoría

El problema de las trazas normales: para capturar un problema **intermitente** tendrías que tener la traza encendida *siempre* y guardar gigabytes, o empezarla *después* de que ocurra… cuando ya es tarde.

El **flight recorder** (la "caja negra" de un avión) resuelve esto: mantiene en memoria una **ventana móvil** con los últimos segundos de traza. Cuando detectas una anomalía, **vuelcas esa ventana** a un archivo: contiene lo que pasó **antes y durante** el incidente.

```go
fr := trace.NewFlightRecorder(trace.FlightRecorderConfig{
    MinAge:   2 * time.Second, // ventana mínima deseada
    MaxBytes: 4 << 20,         // tope de memoria (tiene PRIORIDAD sobre MinAge)
})
fr.Start()
defer fr.Stop()
...
if lento {
    f, _ := os.Create("incidente.trace")
    fr.WriteTo(f)              // vuelca la ventana actual
}
```

### Reglas y detalles

- Solo puede haber **un** flight recorder activo a la vez (pero puede convivir con `trace.Start`).
- `MinAge` es un mínimo orientativo; `MaxBytes` es una cota "de pista" (no garantiza el tamaño exacto de lo escrito).
- La ventana puede ser **más corta** que `MinAge` si el programa acaba de arrancar (en este ejercicio: 1.1 s, porque el primer incidente ocurre ~1.1 s tras empezar).
- El volcado se debe **limitar** (una vez por incidente, con enfriamiento): si cada petición lenta vuelca 1 MB, el diagnóstico agrava el problema.
- Marca el momento con `trace.Logf(ctx, "alerta", ...)`: aparece en la traza y ayuda a localizar el incidente.
- Disparadores típicos: latencia por encima de un umbral, `panic` recuperado, cola creciendo, muchas goroutines, *timeout* upstream.
- Alternativa externa (anterior a 1.25): `golang.org/x/exp/trace.FlightRecorder`.

### Cuándo usarlo

Latencias de cola (*p99*) esporádicas, congelaciones breves, picos que no se reproducen en pruebas. Es **complementario** a las métricas: ellas dicen *que* algo falla; el recorder dice *por qué*.

## Qué hace el programa

Un "servicio" con una base de datos protegida por **un mutex global**. Un goroutine de **mantenimiento** toma ese mutex durante 300 ms cada 800 ms (por ejemplo, una compactación que hace E/S con el lock tomado): es la causa oculta. 8 clientes atienden peticiones durante 3 s; cada petición es una tarea con la región `consulta-bd`.

La primera petición que tarda más de 150 ms **vuelca la ventana** a un archivo temporal, que luego se analiza con `internal/traza`.

```bash
go run .
go test -race .
go test -bench . -benchtime=2s -count=3 -run xxx
```

## Resultado de referencia

```
¡incidente! pedido de 302ms: ventana volcada (272 KB)
peticiones: 8848, lentas (> 150ms): 24, mantenimientos: 3

== análisis de la ventana volcada ==
eventos: 54462 | duración de la traza: 1.103s | goroutines observadas: 26 | máx. goroutines listas a la vez: 9

ESPERAS (Waiting) POR MOTIVO
  sync           8.014s       <- espera por el mutex: domina
  sleep          2.196s
  chan receive   2.103s

REGIONES
  consulta-bd     3372    7.311s    2.168ms media    302.102ms máx.   <- 0.2 ms de trabajo real

TAREAS
  pedido          3371    7.011s    2.08ms media     302.103ms máx.
mensajes trace.Log: 1                                                  <- la marca de alerta
```

**Coste** (microbenchmark que crea una goroutine por operación, un caso exigente para el planificador):

```
Sin recorder:  181.5   188.4   192.2 ns/op
Con recorder:  204.0   203.0   189.5 ns/op        <- ≈ 6-8 % más
```

## Conclusiones

- La ventana volcada contiene el incidente y su historia reciente: **272 KB** para ~1 s de actividad intensa de 8 clientes (54 mil eventos).
- El análisis lo explica sin abrir un navegador: la región `consulta-bd` pasa de 0.2 ms de trabajo a **2.17 ms de media y 302 ms de máximo**, y el motivo de espera dominante es `sync` (8 s acumulados): la causa es un **mutex retenido** durante el mantenimiento.
- Es más difícil de ver en un perfil de CPU (el proceso estaba casi ocioso mientras el lock estaba retenido).
- El coste es del orden de un 6-8 % en este caso extremo; en servicios típicos suele ser menor que el de una traza completa porque nunca escribe a disco hasta que se pide.

## Primera versión descartada

La primera versión usaba un servidor HTTP real (`httptest`). La traza resultante contenía miles de goroutines de red/`select` de la pila HTTP y **tapaba** la señal (esperas por `network` y `select` de 18 s). Se simplificó llamando al "handler" directamente para que la lección (el mutex retenido) fuera nítida. En un servicio real conviene **filtrar por tarea o región** (ver el visor, "User-defined tasks").

## Ejercicios propuestos

1. Cambia `MaxBytes` a 64 KB y observa cómo se acorta la ventana (`duración de la traza`).
2. Corrige la causa (soltar el lock durante la E/S del mantenimiento) y comprueba que ya no hay incidente.
3. Añade enfriamiento: como máximo un volcado por minuto, y limita a 5 archivos con rotación.
4. Dispara el volcado desde un `recover` de pánico en lugar de por latencia.
