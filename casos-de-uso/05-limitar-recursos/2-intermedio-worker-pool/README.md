# Caso 5.2 — Pool de workers y cola acotada  ·  `intermedio`

> **Problema:** Un servicio recibe más trabajo del que puede procesar. Cuando la cola está llena hay tres políticas posibles —bloquear al productor, rechazar o descartar— y elegir una ES una decisión de diseño.

## Qué features usa y por qué

| Feature de Go | Papel en este caso |
|--|--|
| N workers fijos | acotan la concurrencia (y la CPU/conexiones usadas) |
| cola con buffer | acota la memoria y la latencia de espera |
| `Enviar(ctx, t)` | bloquea hasta haber sitio o cancelar: **contrapresión** hacia el productor |
| `Intentar(t)` con `select/default` | no bloquea: devuelve `false` si está llena → «responde 503» |

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ Hay productores que pueden frenarse (componentes internos) → bloquear.
- ✅ En el borde del sistema (HTTP, API pública) → rechazar rápido para no acumular latencia.
- ✅ Telemetría o vídeo en directo → descartar lo más viejo.
- ❌ Cola «infinita»: esconde la sobrecarga hasta que se acaba la memoria y la latencia es enorme.
- ❌ Workers ilimitados («una goroutine por petición» sin freno) frente a un recurso limitado.

## Cómo ejecutarlo

```bash
go run .
go test -race -v .
```

## Qué observar (salida real, Apple M1, Go 1.26.5)

```
Rechazar: de 20 envíos instantáneos → 5 aceptadas, 15 rechazadas (503). Ejecutadas: 5      ← entre 5 y 8; depende de cuántas tomen ya los workers
Bloquear: 20 envíos aceptados todos; el productor tardó 80ms en enviar (frenado), todo terminó a los 150ms
```

## Errores típicos

- Cola sin límite → latencia que crece sin parar (*bufferbloat*).
- `Cerrar()` mientras otros envían → pánico (para eso, el caso 4.3).
- Workers que no vuelven al `range` por un `return` olvidado en la tarea.
- Medir el pool sólo por su rendimiento: la métrica clave es la **profundidad de la cola** y la latencia de espera.

## Profundiza en el curso

- [pool de workers a fondo](../../../08-patrones/01-worker-pool)
- [cola acotada y pérdidas (barbero dormilón)](../../../15-diseno-avanzado/04-barbero)
- [contrapresión en HTTP](../../../15-diseno-avanzado/10-http-backpressure)
- [implementar la cola acotada](../../../15-diseno-avanzado/05-cola-y-ring)
- [capacidad y 503 en `jobq`](../../../proyecto-final)

## Prueba tú

1. Añade `Descartar`: si está llena, saca el más viejo y mete el nuevo.
2. Expón `len(cola)` como métrica y dibújala bajo carga.
3. Cambia 3 workers por 1 y por 30: ¿cuándo deja de mejorar?
