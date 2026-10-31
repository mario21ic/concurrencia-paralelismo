# AGENTS.md — guía para trabajar en este repositorio

Curso autodidacta de **Concurrencia y Paralelismo en Go 1.26**, en español. Cada módulo es una carpeta con ejercicios ejecutables, tests y documentación. El sílabo completo está en `README.md` (raíz).

## Estado actual

Los módulos **01 a 15 están implementados**, cada uno con `README.md` de módulo y `README.md` por ejercicio.

| Módulos | Tema |
|--|--|
| 01-05 | fundamentos, goroutines, channels, select/timers, context |
| 06-07 | sync, atomic |
| 08-10 | patrones, iteradores, errores |
| 11-13 | testing (`synctest`, `goleak`), profiling, rendimiento |
| 14 | internals del runtime (G-M-P, syscalls, preemption, stacks, channels, mutex) |
| 15 | diseño avanzado (deadlock, clásicos, estructuras, lock-free, APIs, HTTP, `database/sql`) |

**Pendiente / ideas** (no es obligatorio hacerlo sin que lo pidan):
- La sección «Estructura sugerida del repositorio» del `README.md` raíz está **desactualizada** (lista `04-select-context`, `09-rendimiento`, `10-avanzado`…; los nombres reales son los de las carpetas actuales).
- `proyectos/` y el «Proyecto final» del sílabo no existen todavía.
- `14-internals/06-canales-y-select`, sección F: la línea del channel `nil` nunca se imprime porque nunca se elige (es lo esperado, pero el mensaje podría mostrar «nil: 0 veces»).

## Entorno

- Módulo Go: `github.com/mario21ic/golang-concurrencia-paralelismo`, `go 1.26.5`, **un único `go.mod` en la raíz**.
- Dependencias: `golang.org/x/sync` (errgroup), `golang.org/x/time` (rate), `golang.org/x/sys` (cpu.CacheLinePad), `golang.org/x/exp` (trace), `go.uber.org/goleak`.
- Plataforma de desarrollo: **macOS (Darwin) arm64, Apple M1, 8 núcleos, líneas de caché de 128 B**. Las cifras de los README salen de ahí.
- **No es un repositorio git** (no hay `.git`): no hay commits ni diff; no asumas que existe historial.

## Comandos

```bash
go vet ./...
gofmt -l .                              # debe salir vacío
go test -race -count=1 ./...            # toda la suite
go test -race -count=1 ./15-diseno-avanzado/...
cd 15-diseno-avanzado/07-skiplist && go run .      # cada ejercicio es `package main`
```

- Ejecuta **siempre con `-race`** al probar.
- Algunos programas terminan **a propósito con `fatal error`** (no es un fallo): `14-internals/02-syscalls-y-netpoller -modo=limite`, `14-internals/05-stacks -modo=desborde`, `15-diseno-avanzado/01-interbloqueos -modo=deadlock-total`.
- Algunos tardan varios segundos por diseño (p. ej. 01-interbloqueos ≈ 8 s por el livelock sin backoff, 10-http-backpressure ≈ 10 s).

## Convenciones del repositorio

**Estructura de un ejercicio** (`NN-nombre/`):

- `main.go` (con `package main`), cabecera con `// Ejercicio N: …` y las líneas `go run .` de los modos.
- Código reutilizable en archivos aparte (`*.go`) para poder probarlo; tests en `*_test.go`.
- `README.md` con **esta estructura**: `# Ejercicio N: título` → `## Teoría` → `## Qué hace el programa` (tabla + comandos) → `## Resultados de referencia` (salida real pegada, con el hardware) → `## Conclusiones` (con números) → `## Ejercicios propuestos` (3-4).
- Los modos se eligen con `flag` (`-modo=...`).

**Estructura de un módulo** (`NN-nombre/README.md`): objetivo → tabla de ejercicios con enlaces → nota de hardware/versión → resumen teórico → «Hallazgos del módulo (aprendidos midiendo)» → «Preguntas de repaso» (≈ 10) → «Lecturas». Usa `13-rendimiento/README.md`, `14-internals/README.md` o `15-diseno-avanzado/README.md` como plantilla.

**Estilo de código:**

- Identificadores, comentarios y mensajes en **español** (`Cola`, `Lanzar`, `Escenario`…). Mantén el idioma del archivo que editas.
- Go moderno: `for range n`, `min`/`max`, `sync.WaitGroup.Go`, `iter.Seq`, `atomic.Int64`/`atomic.Pointer[T]`, genéricos; `testing/synctest` y `t.Context()` en tests.
- Comentarios que explican el **porqué** (la teoría que ilustra), no el qué.
- Antes de dar por bueno un archivo: `gofmt -l`, `go vet` y `go test -race`.

## Reglas del proyecto sobre resultados

1. **Los números de los README deben salir de ejecutar el programa**, en esta máquina. No inventes cifras ni las «redondees» a un valor que no apareció. Si ejecutas de nuevo y salen otras, actualiza el README o aclara que varían.
2. Si un resultado contradice lo esperable (p. ej. el ring MPMC más lento que el channel; las estructuras lock-free no ganan al mutex), **déjalo y explícalo**; no lo maquilles.
3. Cuando afirmes algo teórico no medido (tamaños, umbrales del runtime), que sea cierto del runtime real y se vea como explicación, no como medición.
4. Los tests deben ser **deterministas**: prefiere `testing/synctest` (reloj virtual) a `time.Sleep` reales; si usas tiempo real, margen holgado y comprueba con `-count=3` o más.

## Trampas conocidas (ya nos mordieron)

- **`testing/synctest`:** una goroutine bloqueada en `sync.Mutex`/`RWMutex` o en E/S de red real **no** cuenta como «bloqueada durablemente», así que el reloj virtual no avanza. Úsalo con channels, `time`, `sync.WaitGroup`, `sync.Cond`. Los sockets reales (`net/http` sobre loopback) van **fuera** de la burbuja.
- **`synctest.Test` entra en pánico** si al terminar quedan goroutines bloqueadas o durmiendo: deja pasar el tiempo o cancela el contexto antes de salir.
- **Conteo de goroutines (`runtime.NumGoroutine`) en tests** es frágil: espera a que se asienten las de tests anteriores (ver `base()` en `09-apis-concurrentes/api_test.go`).
- **Timeouts simultáneos son una carrera:** dos goroutines con el mismo plazo pueden verse «salvar» una a otra (caso de `TransferirConPlazo`). Afirma invariantes (conservación de saldos), no el resultado exacto.
- **`goleak.VerifyTestMain`** está en varios paquetes: cualquier goroutine que sobreviva a un test lo hace fallar; los clientes HTTP necesitan `CloseIdleConnections()` o filtros (`IgnoreAnyFunction`).
- **`go vet`** rechaza `fmt.Println("…\n")` (salto de línea redundante): usa `Println` extra.
- **`sed -i` en macOS** exige `sed -i ''`; para ediciones multilínea usa `python3` o la herramienta Edit.
- **Detector de carreras:** los tests de estructuras lock-free usan campos atómicos para no producir falsos positivos; no sustituyas un `atomic` por un campo normal «porque funciona».
- **Preemption / latencia:** `time.Sleep(1ms)` con un acaparador en el mismo P tarda ≈ 25 ms (sysmon); no pongas umbrales de latencia más finos en tests.
- **Los tests de rendimiento** (comparaciones de tiempos) usan umbrales holgados (≥ 2×). Si fallan en otra máquina, relaja el umbral en vez de quitar la comprobación.

## Cómo añadir o modificar un ejercicio

1. Lee el README del módulo y un ejercicio vecino para copiar el tono y el nivel de detalle.
2. Escribe primero el código reutilizable + tests; después `main.go` con las mediciones.
3. `gofmt -l`, `go vet`, `go test -race -count=3` del paquete.
4. Ejecuta `go run .` y pega los resultados **reales** en el README del ejercicio.
5. Añade la fila a la tabla del README del módulo y, si procede, un hallazgo en «Hallazgos del módulo».
6. No toques los módulos ya terminados salvo para corregir errores.

## Notas de contexto del usuario

- Idioma de trabajo: **español**.
- Ritmo habitual: se pide «implementa el módulo N» y se espera código + tests + README por ejercicio + README del módulo, todo verificado.
- Si una ejecución tarda mucho o se interrumpe (p. ej. reinicio del equipo), revisa qué carpetas están vacías o sin README antes de rehacer nada: ya pasó con el módulo 14 (los submódulos 06-08 quedaron vacíos).
