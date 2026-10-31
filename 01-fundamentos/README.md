# Módulo 1. Fundamentos teóricos

**Objetivo:** distinguir concurrencia de paralelismo y entender el modelo de Go antes de escribir código concurrente serio.

## Cómo usar este módulo

Cada carpeta es un ejercicio ejecutable con su propio `README.md` (teoría + qué observar):

| # | Carpeta | Tema |
|---|---------|------|
| 1 | [01-concurrencia-vs-paralelismo](01-concurrencia-vs-paralelismo) | Concurrencia ≠ paralelismo |
| 2 | [02-cpu-vs-io-bound](02-cpu-vs-io-bound) | CPU-bound vs I/O-bound |
| 3 | [03-hilos-vs-goroutines](03-hilos-vs-goroutines) | Procesos, hilos e hilos "verdes" |
| 4 | [04-csp](04-csp) | Modelo CSP |
| 5 | [05-amdahl](05-amdahl) | Ley de Amdahl |
| 6 | [06-memory-model](06-memory-model) | Memory model y happens-before |

```bash
cd 01-fundamentos/01-concurrencia-vs-paralelismo && go run .
```

> Los tiempos de los README son de un Mac con 8 núcleos (arm64, Go 1.26.5). Los tuyos variarán, pero las **proporciones** deben ser similares.

## Resumen teórico

- **Concurrencia** es *estructura*: componer un programa como tareas independientes que pueden avanzar de forma intercalada. **Paralelismo** es *ejecución*: varias tareas corriendo literalmente al mismo tiempo en distintos núcleos. La concurrencia habilita el paralelismo, pero no lo garantiza.
- La carga determina qué ayuda: en **I/O-bound** la concurrencia ayuda aun con un solo núcleo; en **CPU-bound** solo ayuda el paralelismo real.
- Un **proceso** tiene su espacio de memoria; los **hilos** de un proceso lo comparten y los planifica el SO; las **goroutines** las planifica el runtime de Go sobre pocos hilos (modelo M:N) y son mucho más baratas.
- Go adopta **CSP**: preferir comunicar por channels a compartir memoria, aunque ofrece ambos.
- La **ley de Amdahl** pone un techo al speedup según la fracción serial del programa.
- El **memory model** define cuándo una escritura es visible para otra goroutine: solo si existe una relación *happens-before* creada por sincronización.

## Ejercicio de cierre: diagrama propio

Dibuja (en papel o en este archivo, con ASCII/Mermaid) una línea de tiempo de 3 tareas A, B, C. Referencia:

```
Secuencial   (1 núcleo)   |AAAA|BBBB|CCCC|

Concurrente  (1 núcleo)   |AA|BB|CC|AA|BB|CC|     <- intercalado, nunca simultáneo

Paralelo     (3 núcleos)  núcleo 1: |AAAAAAAA|
                          núcleo 2: |BBBBBBBB|
                          núcleo 3: |CCCCCCCC|     <- simultáneo
```

Responde por escrito: ¿en qué caso del ejercicio 1 el tiempo total no mejoró con concurrencia? ¿Por qué?

## Preguntas de repaso

1. ¿Puede haber concurrencia sin paralelismo? ¿Y paralelismo sin concurrencia en el diseño del programa?
2. ¿Por qué 10 000 goroutines son viables y 10 000 hilos de SO suelen no serlo?
3. Si el 10 % de un programa es serial, ¿cuál es el speedup máximo con infinitos núcleos?
4. ¿Qué relación *happens-before* garantiza un `Unlock` seguido de un `Lock`?

## Lecturas

- [The Go Memory Model](https://go.dev/ref/mem)
- Rob Pike, *Concurrency Is Not Parallelism* (charla)
- C. A. R. Hoare, *Communicating Sequential Processes* (1978)
