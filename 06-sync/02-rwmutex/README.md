# Ejercicio 2: `RWMutex`

## Teoría

`sync.RWMutex` permite **muchos lectores simultáneos** (`RLock`/`RUnlock`) **o un único escritor** (`Lock`/`Unlock`). La esperanza: si hay muchas lecturas, no se bloquean entre sí.

Pero cuesta más que un `Mutex`:

- Cada `RLock` hace una operación atómica sobre un contador **compartido**: con muchos núcleos, la línea de caché rebota entre ellos y los lectores se serializan en el hardware aunque no en el lock.
- Si la sección crítica es muy corta, la ganancia de paralelizar lectores se pierde en ese coste.
- Un escritor pendiente **bloquea a los nuevos lectores** (evita su inanición).

### Reglas

1. **No se puede promover** `RLock` → `Lock` (deadlock): suelta y vuelve a tomar, y revalida el estado.
2. **No hagas `RLock` recursivo.** Si entre el primer y el segundo `RLock` llega un escritor, el segundo lector espera al escritor, y el escritor espera al primer lector que ya tienes: deadlock.
3. `RUnlock` solo para `RLock`, y `Unlock` para `Lock`.
4. Mide: no asumas que `RWMutex` es más rápido que `Mutex`.

## Qué hace el programa

1. Un almacén `map[int]int` con `Mutex` y otro con `RWMutex`; 8 goroutines × 200 000 operaciones con 50 %, 90 %, 99 % y 100 % de lecturas. Se repite 5 veces alternando el orden y se toma el mejor tiempo.
2. Dos cargas: sección crítica casi vacía y sección crítica larga (lectura costosa).
3. Demuestra el deadlock del `RLock` recursivo.

```bash
go run .
```

## Resultado de referencia

```
Sección crítica CORTA (lookup de map) (8 goroutines)
  % lecturas        Mutex      RWMutex   RW/Mutex
         50%        217ms         93ms      0.43x
         90%        208ms         69ms      0.33x
         99%        198ms         97ms      0.49x
        100%        171ms        152ms      0.89x

Sección crítica LARGA (≈ 2000 iteraciones)
         50%       1.324s       1.332s      1.01x
         90%       1.315s        977ms      0.74x
         99%       1.316s        531ms      0.40x
        100%       1.304s        247ms      0.19x

DEADLOCK: el lector espera al escritor y el escritor espera al lector (RLock recursivo)
```

## Conclusiones (lee esto con cuidado)

- **Resultado sorprendente:** en esta máquina `RWMutex` **no fue más lento** que `Mutex` ni siquiera con sección crítica corta; incluso ganó con 50 % de escrituras. La regla popular "RWMutex pierde con secciones cortas" **no se cumplió aquí**. Con solo 8 goroutines en un M1, `Mutex` sufre mucho por contención (parada y reactivación de goroutines), y `RWMutex` reparte mejor la espera.
- La regla sí tiende a cumplirse con más núcleos (decenas de CPUs) y secciones críticas mínimas, donde el contador atómico de lectores se convierte en el cuello de botella. **No puedo afirmar que sea tu caso: mídelo.**
- Lo que sí se confirma: la ventaja crece cuando las lecturas son **caras** y dominantes (sección larga, 100 % lecturas: 0.19x → 5 veces menos tiempo).
- Con 50 % de lecturas y sección larga no hay beneficio (1.01x).
- El `RLock` recursivo produce deadlock real, no teórico.

## Ejercicios propuestos

1. Ejecuta con `GOMAXPROCS=2` y con `GOMAXPROCS=1`: ¿se mantiene la ventaja?
2. Si tienes una máquina con más núcleos, repite y compara.
3. Reescribe `reentrada` con un método interno que asume el lock tomado para eliminar el `RLock` anidado.
4. Sustituye el map por un `atomic.Pointer` a un map inmutable (copia al escribir) y compara (adelanto del módulo 7).
