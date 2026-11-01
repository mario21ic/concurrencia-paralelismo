# Concepto 2 · Compartir estado entre goroutines sin romperlo

> **Pregunta guía:** Varias goroutines leen y escriben el mismo dato. ¿Cómo evito que se pisen, y qué herramienta es la adecuada?

La memoria compartida es la fuente de los errores más difíciles de reproducir (**carreras de datos**). Primero: ¿de verdad hace falta compartir? Si no (resultado por índice, estado local y fusionar al final, pasar el dato por un channel), no hay nada que proteger. Si hace falta, estas son las opciones.

## Escalera de casos

| Nivel | Caso | Idea | Feature principal |
|--|--|--|--|
| 1 · básico | [1-basico-mutex](./1-basico-mutex) | 1 000 goroutines incrementan un contador | `sync.Mutex`, `sync/atomic` |
| 2 · intermedio | [2-intermedio-cache](./2-intermedio-cache) | caché que se lee mucho y evita la estampida | `sync.RWMutex`, `singleflight` |
| 3 · avanzado | [3-avanzado-contadores](./3-avanzado-contadores) | métricas de millones de incrementos/s: no compartir | `atomic`, fragmentación, `CacheLinePad` |

## Cómo elegir

| El estado es… | Usa |
|--|--|
| **un número** o puntero con operaciones simples (sumar, comparar-e-intercambiar) | `sync/atomic` |
| una **estructura** o varias variables que cambian juntas | `sync.Mutex` |
| se **lee mucho** y se escribe poco, secciones de lectura largas | `sync.RWMutex` (mide: a veces `Mutex` gana) |
| un mapa con claves que se escriben una vez y se leen mucho, o conjuntos disjuntos por goroutine | `sync.Map` |
| una inicialización que debe ocurrir **una vez** | `sync.Once` / `sync.OnceValue` |
| objetos temporales para **reutilizar** y aliviar al GC | `sync.Pool` |
| una condición que hay que esperar con estado compartido | `sync.Cond` (casi siempre es mejor un channel) |
| millones de actualizaciones/s al mismo dato | **no compartir**: estado local o fragmentado, combinar al leer |
