# Ejercicio 2: operaciones no bloqueantes con `default`

## Teoría

Añadir `default` a un `select` lo vuelve **no bloqueante**: si ningún caso puede proceder en ese instante, ejecuta `default`.

Usos legítimos:

| Patrón | Para qué |
|--|--|
| *try-receive* | "¿hay algo? si no, sigo" |
| *try-send* | "¿cabe? si no, sigo" |
| *load shedding* | descartar trabajo cuando la cola está llena, en lugar de frenar al productor |
| *polling* ocasional | comprobar cancelación en medio de un cálculo largo |

Cuidado:

- **Espera activa:** `for { select { case v := <-ch: ...; default: } }` consume 100 % de CPU. Si necesitas esperar, quita el `default`.
- `len(ch)` no sirve para decidir si enviar: entre consultar y enviar otra goroutine puede cambiar el estado (condición de carrera lógica). `select` con `default` es **atómico**.
- Descartar silenciosamente es una decisión de diseño: cuenta o registra lo descartado.

## Qué hace el programa

1. `tryRecv` sobre un channel vacío.
2. `trySend` 4 veces sobre un buffer de 2.
3. Cola de capacidad 5 con 20 eventos: acepta 5, descarta 15.

```bash
go run .
```

## Resultado de referencia

```
recibir de vacío: false
trySend(1) = true
trySend(2) = true
trySend(3) = false
trySend(4) = false
recibir: 1 true
cola: 5 aceptados, 15 descartados
```

## Conclusiones

- `default` convierte bloquear en *decidir*.
- Es la forma correcta de "enviar si cabe".

## Ejercicios propuestos

1. Cambia el *load shedding* para descartar el elemento **más antiguo** y encolar el nuevo (pista: `try-recv` seguido de `send`).
2. Mide con `top` el uso de CPU de la espera activa y compáralo con la versión bloqueante.
3. Implementa `Semaphore.TryAcquire()` con un channel con buffer.
