# Ejercicio 2: bucles `CompareAndSwap`

## Teoría

El patrón de **actualización optimista**:

```go
for {
    viejo := v.Load()        // 1. leer
    nuevo := f(viejo)        // 2. calcular (sin efectos secundarios)
    if v.CompareAndSwap(viejo, nuevo) { // 3. publicar si nadie cambió v
        return
    }
    // 4. alguien se adelantó: volver a empezar con el valor nuevo
}
```

- Si hay poca contención, casi siempre acierta a la primera: más barato que un lock.
- Con mucha contención, reintenta muchas veces y malgasta CPU.
- `f` debe ser **pura** (puede ejecutarse varias veces y su resultado puede descartarse).
- Garantiza progreso del sistema (*lock-free*), pero no equidad: una goroutine concreta podría reintentar mucho tiempo.

### Tres usos del ejercicio

1. **Máximo concurrente** (`registrarMax`): si `v <= actual` no hay nada que hacer; si no, CAS.
2. **Retirada "si hay fondos"** (`Saldo.Retirar`): comprobar y restar en un único CAS. Es la versión atómica del *check-then-act* del módulo 6, ejercicio 7. La condición va **dentro** del bucle y se vuelve a evaluar con el valor recién leído.
3. **Spinlock** (`Spinlock`): un `Bool` y un CAS. Funciona, pero un goroutine que gira malgasta CPU y el scheduler no sabe que espera; con `Gosched` mejora algo. **Usa `sync.Mutex`**, que sabe aparcar la goroutine.

## Qué hace el programa

```bash
go run .
go run -race .
```

## Resultado de referencia

```
máximo con CAS: 999984 | máximo real: 999984 | coincide: true
saldo 1000, 500 intentos de retirar 7: éxitos=142 (máx. posible 142), saldo final=6
spinlock: contador = 100000 (esperado 100000)
```

## Conclusiones

- 100 000 valores con 100 goroutines: el máximo coincide con el cálculo secuencial.
- Hubo 500 intentos de retirar 7; exactamente 142 tuvieron éxito (1000/7) y el saldo nunca bajó de 0. El resto fueron rechazados.
- El spinlock es correcto, pero no es una buena idea en producción.

## Ejercicios propuestos

1. Implementa un limitador de tasa tipo *token bucket* con un CAS sobre el contador de tokens.
2. Cuenta cuántos reintentos hace `registrarMax` con 100 goroutines y con 2.
3. Mide el spinlock frente a `sync.Mutex` con `GOMAXPROCS=1`: razona la diferencia.
