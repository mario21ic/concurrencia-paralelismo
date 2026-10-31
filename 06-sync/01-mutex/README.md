# Ejercicio 1: `Mutex` e invariantes

## Teoría

`sync.Mutex` garantiza que solo una goroutine está dentro de la sección crítica. Además crea **happens-before**: todo lo escrito antes de `Unlock` es visible tras el siguiente `Lock` (módulo 1, ejercicio 6).

### Qué se protege: invariantes

Una `Cuenta` tiene la regla *saldo ≥ 0*. Cada método toma el lock, comprueba y modifica **en un solo bloque**, de modo que desde fuera el invariante nunca parece violado. El mutex va junto al dato y los campos no son exportados.

### Reglas prácticas

- `Lock(); defer Unlock()`: seguro ante `return` anticipado y `panic`.
- **No es reentrante**: llamar a un método que toma el lock desde otro método que ya lo tiene bloquea para siempre. Patrón: métodos públicos que toman el lock y métodos internos (`depositarLocked`) que asumen que ya está tomado.
- **Zero value utilizable**: `var mu sync.Mutex` ya está listo.
- **No copiar** un mutex en uso (`go vet` → `copylocks`).
- `TryLock()` intenta sin bloquear. Casi siempre indica un problema de diseño; útil en casos concretos (limitar trabajo opcional).

### Varios locks: orden global

`Transferir(A, B)` necesita bloquear **ambas** cuentas. Si una goroutine hace A→B (bloquea A, luego B) y otra B→A (bloquea B, luego A), cada una espera lo que tiene la otra: deadlock. Solución: **todas** toman los locks en el mismo orden (aquí, por `id` ascendente), independientemente de la dirección.

```
Mal:   G1: lock A → lock B      G2: lock B → lock A      ✗ ciclo
Bien:  G1: lock A → lock B      G2: lock A → lock B      ✓ (por id)
```

## Qué hace el programa

```bash
go run .                  # 1000 transferencias concurrentes en ambos sentidos
go run . -modo=deadlock   # orden inconsistente => deadlock
go run -race .
```

## Resultado de referencia

```
a=7 b=1993 total=2000 (esperado 2000)
transferencias: 1000, rechazadas: 1
TryLock con el lock tomado: false
TryLock con el lock libre:  true

# -modo=deadlock
fatal error: all goroutines are asleep - deadlock!
goroutine 1 [sync.WaitGroup.Wait]:
```

## Conclusiones

- El **total** se conserva (2000) y `a` nunca queda negativa: la `a` terminó casi en 0 y la transferencia que no tenía fondos fue **rechazada** (1 de 1000), no ejecutada.
- Con orden consistente no hay deadlock aunque las transferencias vayan en sentidos opuestos.
- El deadlock del modo `deadlock` lo detecta el runtime solo porque todas las goroutines están bloqueadas; en un servidor real quedaría colgado en silencio.

## Ejercicios propuestos

1. Añade `Cuenta.Transferir` como método y observa por qué es tentador (y erróneo) llamar a `Depositar` y `Retirar` desde él.
2. Copia una `Cuenta` por valor y ejecuta `go vet`.
3. Añade un campo `historial []string` y asegúrate de que se actualiza dentro del mismo lock que el saldo.
4. Usa `-race` con una versión que lea `saldo` sin el lock.
