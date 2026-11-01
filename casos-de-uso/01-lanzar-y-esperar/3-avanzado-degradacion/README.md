# Caso 1.3 — Panel con servicios opcionales y plazo global  ·  `avanzado`

> **Problema:** El panel de un usuario consulta 3 servicios: usuario y pedidos son **esenciales**; recomendaciones es **opcional**. Si el opcional tarda o falla, el panel debe salir igualmente avisando de que está incompleto.

## Qué features usa y por qué

| Feature de Go | Papel en este caso |
|--|--|
| `errgroup` | esperar a los tres y propagar el error de los esenciales |
| `context.WithTimeout` anidado | un plazo global del panel y otro, más corto, para el opcional (un servicio lento no consume todo el presupuesto) |
| Decisión de diseño | qué errores **abortan** (devolver `err`) y cuáles se **anotan** (devolver `nil` y registrar en `Incompleto`) |
| `sync.Mutex` | solo para el slice `Incompleto` (campos distintos de la estructura no necesitan cerrojo) |

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ Agregas datos de varios backends y alguno es prescindible (**degradación elegante**).
- ✅ Quieres una latencia máxima garantizada para el conjunto.
- ❌ Todas las partes son imprescindibles → caso 1.2 es suficiente.
- ❌ Necesitas el resultado parcial de TODAS aunque fallen (usa un slice de resultados con error por posición).

## Cómo ejecutarlo

```bash
go run .
go test -race -v .
```

## Qué observar (salida real, Apple M1, Go 1.26.5)

```
todo bien                              80ms  err=<nil>  panel={Usuario:ana Pedidos:[#1 #2] Recomendaciones:[libro] Incompleto:[]}
recomendaciones lentas (opcional)     200ms  err=<nil>  panel={... Recomendaciones:[] Incompleto:[recomendaciones: context deadline exceeded]}
recomendaciones fallan (opcional)      80ms  err=<nil>  panel={... Incompleto:[recomendaciones: 503]}
pedidos fallan (esencial)              20ms  err=BD caída  panel={Usuario: Pedidos:[] ...}
```

## Errores típicos

- Dejar que el opcional **cancele** a los esenciales al devolver su error (por eso devuelve siempre `nil`).
- Darle al opcional el mismo plazo que al total: un opcional lento se comería todo el presupuesto.
- Devolver un panel «a medias» **sin avisarlo**: el llamante no sabe que falta algo (`Incompleto`).
- Escribir `p.Incompleto` desde varias goroutines sin cerrojo.

## Profundiza en el curso

- [`errgroup` y errores](../../../10-errores/01-errgroup)
- [plazos con `context`](../../../05-context/02-timeout-deadline)
- [fallar pronto y degradar bajo carga](../../../15-diseno-avanzado/10-http-backpressure)
- [reunir varios errores](../../../10-errores/02-errors-join)

## Prueba tú

1. Añade un 4.º servicio opcional y un contador de «partes que faltan».
2. Cambia el plazo global a 100 ms: ¿qué parte falla primero?
3. Devuelve un `Panel` parcial **junto con** el error cuando falle un esencial (qué se pudo obtener).
