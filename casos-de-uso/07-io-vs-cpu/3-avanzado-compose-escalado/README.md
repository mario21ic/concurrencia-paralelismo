# Caso 7.3 — Escalar con Docker Compose: N contenedores en paralelo  ·  `avanzado`

> **Problema:** Un trabajo de CPU ya no cabe en un proceso (o quieres aislarlo). ¿Cómo lo reparto entre varios **contenedores** que se ejecutan a la vez, y cuánto acelera de verdad?

Es la versión ejecutable de [7.2](../2-avanzado-contenedores) (que explica el porqué) y mucho más pequeña que el [`proyecto-final`](../../../proyecto-final): aquí no hay cola ni coordinador, solo workers y un cliente.

## Qué hay

| Pieza | Qué es |
|--|--|
| `worker` | el mismo binario en modo `-modo=servidor`: `GET /trabajo?iter=N` hace N iteraciones de CPU pura y responde con el resultado y su nombre (`X-Worker`) |
| `cliente` | modo `-modo=cliente`: envía 800 tareas con 64 peticiones simultáneas y mide el tiempo; está en el perfil `carga` de Compose |
| `compose.yaml` | cada worker con `cpus: 1.0`; las réplicas se eligen con `--scale worker=N` |
| `Dockerfile` | multietapa (golang → alpine), contexto = raíz del repo (allí está `go.mod`) |
| `scripts/escalar.sh` | levanta 1, 2, 4, 8 workers y mide cada vez |

Features de Go: `net/http` (servidor y cliente), `signal.NotifyContext` + `Server.Shutdown` (apagado ordenado), `WaitGroup.Go`, un canal de tareas con N goroutines, y un `DialContext` propio. Features de Compose: `--scale`, `healthcheck` + `--wait`, `profiles`, límite de CPU por contenedor.

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ La carga **CPU** supera a una máquina, o quieres escalar y desplegar los workers por separado de la API.
- ✅ Para **medir** antes de comprometerte con una arquitectura distribuida: este experimento da el techo del escalado sin red compleja.
- ❌ Si el trabajo por petición es diminuto: la red y las conexiones cuestan más que el cálculo (agrúpalo en lotes).
- ❌ Aquí **no hay** reintentos, cola ni tolerancia a fallos (si un worker cae, el cliente falla). Eso lo resuelve el [proyecto final](../../../proyecto-final).

## Cómo ejecutarlo

```bash
cd casos-de-uso/07-io-vs-cpu/3-avanzado-compose-escalado

./scripts/escalar.sh                  # speedup con 1, 2, 4 y 8 workers (≈ 1 min)
./scripts/escalar.sh "1 3 6"          # otras réplicas

# a mano:
docker compose up -d --build --scale worker=4 --wait worker
docker compose run --rm --no-deps cliente
docker compose down

# sin Docker, un worker local:
go run . -modo=servidor &
go run . -modo=cliente -url=http://localhost:8080 -tareas=100 -iter=2000000
go test -race -v .
```

Variables para cambiar la carga: `TAREAS` (800), `ITER` (10 000 000 iteraciones por tarea), `CONC` (64) y `SERIAL` (0; ver [8.4](../../08-medir-el-paralelismo/4-avanzado-amdahl-contenedores)). Usa `--no-deps` en `run`: sin él Compose vuelve a evaluar `worker` y puede cambiarte el número de réplicas.

## Qué observar (salida real, Apple M1, Docker Desktop con 8 CPU, Go 1.26.5)

Reparto entre 3 workers (carga pequeña, 60 tareas):

```
cliente: 60 tareas, 300000000 iteraciones de CPU en total, 64 peticiones simultáneas, parte serial 0 %
serial 0s | paralela 227ms | total 227ms | 265 tareas/s
reparto entre 3 workers: 8f762a69d1dd=20 a8e6f12cf260=20 b5d6c135cf85=20
```

Escalado (`./scripts/escalar.sh "1 2 4 8 1"`, 800 tareas de 10 M iteraciones):

```
workers  total      tareas/s     speedup   eficiencia workers que respondieron
1        18.92 s    42           1.00x     100%       1
2        8.97 s     89           2.11x     105%       2
4        4.62 s     173          4.10x     102%       4
8        3.15 s     254          6.00x     75%        8
1        19.18 s    42           0.99x     99%        1
```

(Otra ejecución completa dio 18.94 s / 8.38 s / 4.61 s / 3.31 s → 2.26×, 4.10×, 5.71×. El `N=1` se repitió en 18.9-19.2 s, así que la variación es de pocas décimas, salvo en el de 8 workers.)

- **Casi lineal hasta 4 contenedores** (≈ 4.1×), y **6.0×** con 8: cada contenedor tiene su propia CPU y su propia memoria, sin cerrojos compartidos. El techo de 8× no se alcanza: el cliente también consume CPU y Docker Desktop reparte sus 8 CPU del M1 entre núcleos de rendimiento y de eficiencia.
- **Eficiencia superior al 100 % con 2 y 4 workers**: no es magia, es la base. Con 1 worker las 64 peticiones simultáneas compiten dentro de un único contenedor limitado a 1 CPU (Go fija `GOMAXPROCS=2`), y 18.9 s es más que los ≈ 16 s que saldrían de 800 tareas de ≈ 20 ms cada una (a partir de los ≈ 2 ms por millón de iteraciones que midió [8.1](../../08-medir-el-paralelismo/1-basico-speedup) directamente en el host). Con más réplicas cada contenedor atiende menos peticiones simultáneas y esa fricción desaparece. Es una explicación plausible, no una medición del throttling.
- La columna final es la comprobación de que **todos los workers trabajaron**: ver el apartado siguiente.

## Un fallo que merece la pena entender

En la primera versión el cliente usaba el nombre `worker` tal cual y **las 60 tareas fueron a un solo contenedor** (`reparto entre 1 workers`), con 3 réplicas vivas. Docker devuelve una IP por réplica, pero el resolvedor de Go las **reordena** (RFC 6724) y siempre elige la misma. Sin cambiar nada más, el escalado habría sido nulo y el programa, aparentemente correcto.

Ahora `dialBalanceado` resuelve el nombre, ordena las IP y reparte las conexiones en round-robin (y se desactivan las conexiones persistentes para que cada petición decida de nuevo). Es *balanceo en el cliente*; en producción lo haría un balanceador (nginx, Envoy, un `Service` de Kubernetes). **Lección general: comprueba siempre que la carga se reparte** (por eso el cliente imprime cuántas respuestas dio cada worker).

## Errores típicos

- Escalar réplicas y no comprobar que reciben tráfico (el caso anterior).
- Medir con pocas tareas: el arranque de los contenedores y la conexión dominan.
- Usar conexiones HTTP persistentes sin balanceo: quedan pegadas a un worker.
- `docker compose run` sin `--no-deps`: puede recrear los workers con otro número de réplicas.
- Olvidar que Docker Desktop reparte sus CPU: `docker info` dice cuántas hay; con 4 no verás 8×.

## Profundiza en el curso

- [7.2 — por qué y cuándo salir del proceso](../2-avanzado-contenedores)
- [8.4 — la ley de Amdahl con estos mismos contenedores](../../08-medir-el-paralelismo/4-avanzado-amdahl-contenedores)
- [proyecto final: coordinador, arriendos, reintentos y tolerancia a fallos](../../../proyecto-final)
- [apagado ordenado](../../04-tiempo-y-cancelacion/3-avanzado-apagado-ordenado): lo que hace el worker al recibir `SIGTERM`
- [`GOMAXPROCS` y cuotas de CPU](../../../02-goroutines/04-runtime-info)

## Prueba tú

1. Ejecuta `./scripts/escalar.sh "1 2 4 8 16"`: ¿qué pasa con 16 workers si Docker tiene 8 CPU?
2. Mata un worker en mitad de la carga (`docker kill casos-escalado-worker-2`): ¿qué hace el cliente? ¿qué le falta para sobrevivir? (spoiler: reintentos, ver [4.2](../../04-tiempo-y-cancelacion/2-intermedio-reintentos))
3. Cambia `ITER` a 100 000 y `TAREAS` a 8 000: ¿sigue escalando o domina la red?
