# Ejercicio 3: lectores–escritores

## Teoría

Un recurso se lee mucho y se escribe poco: **muchos lectores a la vez** son seguros; un **escritor necesita exclusividad** (ni lectores ni otros escritores). La pregunta no es la exclusión, sino **quién tiene preferencia cuando hay conflicto**:

| Política | Regla | Riesgo |
|--|--|--|
| **Preferencia de lectores** | si ya hay lectores dentro, entran más aunque un escritor espere | **inanición del escritor** |
| **Preferencia de escritores** | si un escritor espera, no entran lectores nuevos | inanición de lectores si llegan escritores sin parar |
| **Justa** (FIFO / torniquete) | todos pasan por un punto común en orden de llegada | menos rendimiento (más sincronización) |

### Las tres implementaciones

- **`PrefLectores`**: el "interruptor" clásico (*lightswitch*): el **primer** lector cierra la sala a los escritores (`sala.Lock`), el **último** la abre. Un escritor solo hace `sala.Lock`.
- **`PrefEscritores`** (con `sync.Cond`): un contador de escritores esperando; `RLock` espera mientras haya alguno. `Broadcast` al liberar.
- **`Justo`** (*torniquete* de *The Little Book of Semaphores*): los lectores pasan por un cerrojo `torno` solo para tocarlo y soltarlo; el escritor lo mantiene **cerrado** desde que llega hasta que acaba. Así los lectores posteriores esperan detrás del escritor y nadie se inanicia.

### `sync.RWMutex` de Go

- Si hay un `Lock()` pendiente, los `RLock()` **nuevos se bloquean** (evita la inanición del escritor).
- **No es reentrante en lectura**: un `RLock` recursivo con un escritor esperando entre medias es un **deadlock** (el escritor espera al primer lector, el segundo `RLock` espera al escritor).
- Cuando un escritor suelta, los lectores bloqueados entran todos de golpe: tiene una política **cercana a la justa**.
- Tiene una sobrecarga fija (un contador atómico compartido por lectores): con secciones críticas muy cortas, un `Mutex` puede ser más rápido que `RWMutex`. Compensa cuando la sección de lectura es **larga** y hay **muchos más lectores que escritores**.

## Qué hace el programa

Con secciones críticas de 200 µs durante 600 ms, mide para cada política lecturas, escrituras y la **máxima espera** de cada tipo, en dos cargas: 8 lectores + 2 escritores, y 2 lectores + 8 escritores.

```bash
go run .
go test -race -v .
```

## Resultados de referencia

```
8 lectores en bucle + 2 escritores
  política                    lecturas escrituras máx. espera lect  máx. espera esc
  preferencia de lectores        20641          2               0s         600.52ms   ← el escritor NO entra
  preferencia de escritores        887       1714         353.32ms            570µs
  justa (torniquete)              5759       1277           1.02ms            760µs
  sync.RWMutex                   10229       1278            470µs            700µs

2 lectores + 8 escritores en bucle
  preferencia de escritores          5       2565         601.45ms            3.5ms    ← los lectores NO entran
  justa (torniquete)               397       2310           3.64ms           2.43ms
  sync.RWMutex                    2561       1286            480µs           3.53ms
```

## Conclusiones

- **La preferencia de lectores maximiza las lecturas (20 641) pero los escritores hacen 2 operaciones en 600 ms** (espera máxima = toda la ejecución). Es el patrón de inanición clásico.
- **La preferencia de escritores solo mueve el problema:** con escritores continuos, 5 lecturas en 600 ms.
- **`sync.RWMutex` no sufre ninguna de las dos inaniciones** (esperas máx. < 4 ms) y es **el doble de rápido** que mi torniquete (10 229 frente a 5 759 lecturas): usa atómicos en lugar de tres mutex. Conclusión práctica: **usa `sync.RWMutex` y no reimplementes**; implementarlo una vez sirve para entender por qué se comporta así.
- **Los tests detectan la inanición con un plazo:** un escritor tiene 300 ms para entrar con 8 lectores solapados; solo `PrefLectores` falla (y el test lo afirma).

## Ejercicios propuestos

1. Añade a `PrefEscritores` un tope: tras k escritores seguidos, deja pasar a los lectores acumulados (política *adaptativa*).
2. Reproduce el deadlock del `RLock` recursivo en `sync.RWMutex` y arréglalo.
3. Compara `sync.Mutex` frente a `RWMutex` variando la duración de la sección de lectura (1 µs, 10 µs, 100 µs).
4. Implementa un cerrojo lector-escritor con `chan` (un token por lector) y mide.
