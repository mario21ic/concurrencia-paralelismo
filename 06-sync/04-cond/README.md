# Ejercicio 4: `sync.Cond`

## Teoría

`sync.Cond` permite que goroutines **esperen una condición** sobre estado protegido por un mutex, y que otras la **avisen** cuando cambia.

```go
c := sync.NewCond(&mu)

// espera
mu.Lock()
for !condicion() { // SIEMPRE for
    c.Wait()       // libera mu, duerme, y al despertar retoma mu
}
// ... usar el estado, con mu tomado ...
mu.Unlock()

// aviso
mu.Lock()
cambiarEstado()
c.Signal()     // despierta a UNA goroutine que espera
// c.Broadcast() // despierta a TODAS
mu.Unlock()
```

### Reglas

1. **`for`, no `if`:** al despertar hay que reevaluar la condición. Otra goroutine puede haberla invalidado, o puede haber *despertares espurios*.
2. Se llama a `Wait` con el mutex **tomado**.
3. `Signal`/`Broadcast` pueden llamarse con o sin el mutex; hacerlo con él es más simple de razonar.
4. Varias `Cond` pueden compartir el mismo mutex (aquí `noVacia` y `noLlena`) para distinguir *quién* debe despertar.
5. `Signal` cuando solo uno puede aprovechar el cambio; `Broadcast` cuando cambia una condición global (flag, apagado).

### ¿Y los channels?

Muchos usos de `Cond` se escriben mejor con channels (`close` = difusión, channel con buffer = cola acotada). `Cond` destaca cuando:

- la condición depende de **estado complejo** que ya vive bajo un mutex, y
- necesitas `Broadcast` repetible (un `close` solo puede hacerse una vez).

Limitación importante: **`Cond.Wait` no admite `context`/timeout**. Si necesitas cancelar la espera, un channel suele ser más simple.

## Qué hace el programa

1. Cola acotada (capacidad 3) con 3 productores × 10 elementos y 2 consumidores: no se pierde ni duplica nada (se compara la suma).
2. 5 goroutines esperan un flag con `Cond`; `Broadcast` las libera a todas.

```bash
go run .
go run -race .
```

## Resultado de referencia

```
cola: 30 elementos consumidos, suma=3165 (esperada 3165)
antes de Broadcast, arrancaron: 0
tras Broadcast, arrancaron: 5
```

## Conclusiones

- Los productores se bloquean cuando la cola está llena y los consumidores cuando está vacía, sin espera activa ni polling.
- `Broadcast` libera a todos de una vez.

## Ejercicios propuestos

1. Cambia `for` por `if` en `Poner` y ejecuta con más consumidores/productores: busca elementos duplicados o pánicos por slice vacío.
2. Reescribe `Cola` con un channel con buffer y compara líneas de código y rendimiento.
3. Añade `Cerrar()` a la cola que despierte (con `Broadcast`) a todos los consumidores y los haga devolver `ok=false`.
