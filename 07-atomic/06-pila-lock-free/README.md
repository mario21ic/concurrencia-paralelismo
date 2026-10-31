# Ejercicio 6: pila lock-free

## Teoría

La **pila de Treiber** (1986) es la estructura lock-free más sencilla: una lista enlazada cuya cabeza es un puntero atómico.

```
cabeza ──► [c] ──► [b] ──► [a] ──► nil

Push(d):  d.siguiente = cabeza; CAS(cabeza, vieja, d)
Pop():    CAS(cabeza, vieja, vieja.siguiente)
```

Ambas operaciones usan un bucle CAS: si otra goroutine cambió la cabeza entre la lectura y el CAS, se reintenta.

### Propiedades

- **Sin locks:** ninguna goroutine puede bloquear a las demás (si una se duerme a mitad de una operación, las otras siguen).
- **Lock-free** (progreso del sistema) pero no *wait-free*: una goroutine concreta puede reintentar indefinidamente.
- **ABA:** en Go el recolector evita que la dirección de `viejo` se reutilice mientras la tienes en una variable, lo que elimina el caso clásico. Ojo si reutilizas nodos (`sync.Pool`, lista libre): el CAS podría tener éxito sobre un nodo "reciclado" con otro `siguiente`.
- Genéricos: `Pila[T any]`.

### ¿Más rápido?

No necesariamente. El mutex es simple y está muy optimizado; un CAS que falla repite el trabajo y produce tráfico de caché. La estructura lock-free aporta **ausencia de bloqueos** (tolerancia a goroutines lentas, sin inversión de prioridad), no velocidad automática.

## Qué hace el programa

4 productores apilan 1..200 000 cada uno y 4 consumidores desapilan hasta sacar todo. Se verifica que la suma coincide (ningún elemento perdido ni duplicado) y se compara con una pila con `Mutex`.

```bash
go run .
go test -race -count=3 .
go test -bench . -run xxx
```

## Resultado de referencia

```
lock-free (CAS)  suma=80000400000 esperada=80000400000 correcto=true en 309ms
mutex            suma=80000400000 esperada=80000400000 correcto=true en 153ms

BenchmarkLockFree-8   264.7 ns/op
BenchmarkMutexPila-8  198.8 ns/op
```

## Conclusiones

- Ambas son **correctas**; los tests pasan con `-race -count=3`.
- La versión **lock-free fue ~2× más lenta** en el programa y ~33 % más lenta en el benchmark. Además cada `Push` asigna un nodo nuevo, mientras que la de mutex usa un slice amortizado.
- Conclusión práctica: **no escribas estructuras lock-free por defecto**. Hazlo cuando un perfil lo justifique, y usa las de la biblioteca estándar o de terceros probadas.
- En el programa los consumidores giran mientras la pila está vacía; es un caso de espera activa que penaliza a ambas versiones.

## Ejercicios propuestos

1. Haz que cada `Push` reutilice nodos con `sync.Pool` y razona si introduces el problema ABA.
2. Implementa una cola lock-free de Michael-Scott (más difícil: dos punteros).
3. Cuenta los reintentos de CAS por operación con un contador atómico y relaciónalos con el número de goroutines.
4. Añade un `Len()` aproximado con un `atomic.Int64`. ¿Por qué es solo aproximado?
