# Ejercicio 2: la cena de los filósofos

## Teoría

Problema de Dijkstra (1965): **n filósofos** en una mesa redonda, **n tenedores** (uno entre cada par). Para comer hace falta coger **los dos** tenedores contiguos. Es el modelo canónico de **adquisición de varios recursos**:

| Elemento | En un sistema real |
|--|--|
| filósofo | goroutine/hilo/transacción |
| tenedor | cerrojo, fila de BD, conexión |
| coger los dos | transacción que bloquea dos filas, transferencia entre dos cuentas |

La solución ingenua —*coger el izquierdo, luego el derecho*— se interbloquea si todos cogen el izquierdo a la vez: **espera circular**.

### Las estrategias

| Estrategia | Idea | Condición de Coffman que rompe | Pega |
|--|--|--|--|
| **Ingenua** | izquierdo → derecho | ninguna | interbloqueo |
| **Ordenada** | siempre el tenedor de **menor número** primero | espera circular | exige un orden global conocido; reparto algo desigual (el filósofo "del cierre del círculo" coge en orden inverso) |
| **Mayordomo** | solo **n−1** filósofos sentados a la vez (semáforo) | el ciclo completo no puede formarse | coordinación central; limita el paralelismo |
| **Asimétrica** | los pares empiezan por la derecha | simetría del ciclo | un patrón fijo, depende de n par/impar |
| *(Chandy–Misra)* | tenedores "sucios/limpios" y peticiones por mensajes | distribuida y justa | más compleja (propuesto abajo) |

### El interbloqueo latente

Un deadlock depende de un entrelazado concreto. Si el tiempo entre coger el 1.º y el 2.º tenedor es casi nulo, el código ingenuo **parece funcionar**; basta que cambie la temporización (más carga, otro hardware) para que falle. **Un test que pasa no demuestra que no haya deadlock**; hay que razonar sobre el orden de adquisición o forzar la ventana (como aquí con `pausa`).

### Equidad (starvation)

Evitar el deadlock no basta: si el algoritmo permite que un filósofo pierda siempre la carrera por los tenedores, hay **inanición**. La métrica `equidad = min/max` de comidas lo mide.

### Tenedores cancelables

Un `sync.Mutex` no se puede abandonar. Aquí cada tenedor es un `chan struct{}` de capacidad 1 que se toma con `select` junto a `ctx.Done()`: así la cena termina limpiamente incluso con el sistema interbloqueado (sin goroutines huérfanas).

## Qué hace el programa

Sienta a 5 filósofos 600 ms con cada estrategia, con una pausa forzada de 20 ms entre el primer y el segundo tenedor (y también sin pausa). Al final repite la estrategia ingenua en cenas cortas para medir la frecuencia del bug latente.

```bash
go run .
go test -race -v .
```

## Resultados de referencia

```
5 filósofos, 600ms de cena, comer = 1 ms

== pausa 20ms entre el 1.º y el 2.º tenedor ==
  ingenua            0 comidas  NUNCA (interbloqueo)
  ordenada          96 [11 23 24 26 12]      equidad 0.42
  mayordomo (n-1)  109 [22 22 22 22 21]      equidad 0.95
  asimétrica        79 [13 14 13 13 26]      equidad 0.50

== sin pausa ==
  ingenua          504 [100 101 101 101 101] equidad 0.99    (¡no se bloqueó!)
  ordenada        1023 [189 192 192 261 189] equidad 0.72
  mayordomo        486 [97 97 97 97 98]      equidad 0.99
  asimétrica       994 [176 176 187 186 269] equidad 0.65

== bug latente: 'ingenua' sin pausa forzada, 100 cenas de 40 ms ==
  comer = 0      → 100 de 100 interbloqueadas
  comer = 100µs  →   0 de 100
  comer = 1ms    →   0 de 100
```

## Conclusiones

- **La estrategia ingenua falla o no según la temporización:** con `comer = 0` se bloquea el **100 %** de las veces; con `comer = 100 µs` el **0 %**. El mismo código.
- **Ordenada y asimétrica evitan el deadlock y rinden el doble en pasadas sin pausa** (≈1000 comidas frente a ≈500) porque permiten que dos filósofos no vecinos coman a la vez; **el mayordomo** es el más **equitativo** (0.95-0.99) pero no más rápido.
- **Hay un precio en equidad:** el orden global deja a un filósofo con muchas más comidas (261 frente a 189) o con muchas menos (11 y 12 frente a 26 con pausa). Evitar el deadlock **no** garantiza justicia.
- La solución en producción casi siempre es **orden global de recursos** (por ID de fila, dirección…) porque no necesita coordinador.

## Ejercicios propuestos

1. Implementa **Chandy–Misra** (tenedores limpios/sucios con peticiones) y compara la equidad.
2. Generaliza a `k` tenedores necesarios por comida (*problema de los k-recursos*): ¿cuál es el límite del mayordomo?
3. Mide con `n = 2, 3, 5, 16, 64` filósofos cómo escala cada estrategia.
4. Cambia el mayordomo por `semaphore.Weighted` de `golang.org/x/sync` y compara.
