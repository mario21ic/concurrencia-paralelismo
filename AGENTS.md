# AGENTS.md — guía para trabajar en este repositorio

Curso autodidacta de **Concurrencia y Paralelismo en Go 1.26**, en español. Cada módulo es una carpeta con ejercicios ejecutables, tests y documentación. El sílabo completo está en `README.md` (raíz).

## Estado actual

Los módulos **01 a 15, `casos-de-uso/` y el proyecto final (`proyecto-final/`) están implementados**, cada módulo con `README.md` de módulo y `README.md` por ejercicio.

| Módulos | Tema |
|--|--|
| 01-05 | fundamentos, goroutines, channels, select/timers, context |
| 06-07 | sync, atomic |
| 08-10 | patrones, iteradores, errores |
| 11-13 | testing (`synctest`, `goleak`), profiling, rendimiento |
| 14 | internals del runtime (G-M-P, syscalls, preemption, stacks, channels, mutex) |
| 15 | diseño avanzado (deadlock, clásicos, estructuras, lock-free, APIs, HTTP, `database/sql`) |

**Pendiente / ideas** (no es obligatorio hacerlo sin que lo pidan):
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

## Proyecto final (`proyecto-final/`)

`jobq`: cola de trabajos distribuida (coordinador + workers + loadgen), opción 2 del sílabo. Lee `proyecto-final/README.md` (decisiones de diseño, API, resultados medidos) antes de tocarlo.

- Paquetes: `internal/{modelo,planificador,api,worker}` y `cmd/{coordinador,worker,loadgen}`. El **corazón** es `internal/planificador` (un `Mutex`, 4 colas FIFO por prioridad con envejecimiento, línea de tiempo `min-heap` con invalidación por generación, `Run(ctx)` como reloj).
- Tests: `cd proyecto-final && make test` (`go vet` + `go test -race`). El planificador se prueba con `synctest` (backoff y caducidad exactos); `api` y `worker` con `httptest` y tiempo real corto.
- Docker: el contexto de build es la **raíz del repo** (allí está `go.mod`): `docker compose -f proyecto-final/compose.yaml …` o desde `proyecto-final/`: `docker compose up -d --build`. Hay un `.dockerignore` en la raíz que solo deja pasar `go.mod`, `go.sum` y `proyecto-final/`.
- **Trampa de Compose:** `docker compose run` vuelve a evaluar la configuración y **recrea el coordinador** si cambió una variable. Si pruebas `CAPACIDAD=100`, expórtala para *todos* los comandos (`export CAPACIDAD=100`), o el coordinador vuelve a los valores por defecto.
- **Trampa de GOMAXPROCS:** con `cpus: 1.0` Go fija `GOMAXPROCS=2` (el mínimo del runtime), no 1.
- Escalado medido con `./scripts/escalado.sh`; deja `docker compose down` al terminar.
- No cambies cifras del README sin volver a ejecutar el experimento correspondiente.

## Casos de uso por concepto (`casos-de-uso/`)

Guía **transversal**: los módulos están ordenados por feature de Go; esta carpeta, por **problema**, para enseñar *qué feature usar y cuándo*. 10 conceptos × (básico, intermedio, avanzado) = 28 casos con código + 1 caso solo documental (7.2, apunta a `proyecto-final/`). Conceptos 1-5 = **concurrencia**; 6-10 = **paralelismo** (6 CPU, 7 E/S vs CPU, 8 medir speedup/Amdahl/memoria, 9 repartir el trabajo, 10 simulaciones por fases con barreras). El usuario pidió reforzar el paralelismo porque al principio la carpeta era casi todo concurrencia: mantén el equilibrio al añadir casos.

- Estructura: `casos-de-uso/NN-concepto/N-nivel-nombre/{main.go,main_test.go,README.md}` + un `README.md` por concepto + el índice `casos-de-uso/README.md` (mapa, árbol de decisión, tabla «feature → cuándo sí/no», ruta de estudio, errores comunes).
- Cada README de caso sigue la plantilla: Problema → Qué features usa y por qué → ¿Cuándo usarlo? ¿Cuándo no? → Cómo ejecutarlo → Qué observar (salida real) → Errores típicos → Profundiza en el curso (enlaces relativos a los módulos) → Prueba tú.
- Los enlaces a módulos son relativos (`../../../NN-modulo/ejercicio`): si renombras una carpeta de módulo, hay que actualizarlos. Comprobación rápida: un script que recorra los `README.md` y verifique que cada enlace relativo existe (así se validó: 0 rotos).
- Al añadir un caso: código + test (`-race`, `synctest` si hay tiempo, `goleak` si hay goroutines de larga vida), salida real pegada en el README, fila en el mapa y, si introduce una feature nueva, en la tabla «feature → cuándo».
- Lecciones de esta carpeta: un test `synctest` falla con «main bubble goroutine has exited but blocked goroutines remain» si una función deja goroutines vivas (haz que esperen a terminar, p. ej. con un canal `fin`); quien crea la fuente de un pipeline debe poder detenerla (`ctx`).
- Casos de **paralelismo** (8-10): son benchmarks, así que sus números dependen de la carga de la máquina (el equipo de desarrollo suele tener videollamadas abiertas). Mide **intercalando** variantes y con «mejor de N» (ver 8.3), repite si algo sale raro (un `k=1` anómalo daba 2.7× con 2 workers), y si una hipótesis tuya no se confirma, **dilo en el README** (9.3, 10.3). Los tests de estos casos comprueban **corrección** (mismo resultado que el secuencial, bit a bit si es posible) y solo comparan tiempos con umbrales muy holgados o con métricas deterministas (9.2 compara el coste en iteraciones, no el reloj).
- Reserva fuera de la medición los búferes grandes (9.3): fallar páginas de cientos de MB puede dominar el tiempo y falsear el speedup.
- La energía **cinética** no se conserva en un sistema gravitatorio; el invariante correcto para probar el N-cuerpos es el momento lineal total (10.3).
