# Caso 7.1 — ¿Cuántos workers?  ·  `básico`

> **Problema:** 256 tareas y un pool de N workers. ¿Qué N elijo? Depende de si la tarea ESPERA (E/S) o CALCULA (CPU). El experimento lo mide con el mismo pool.

## Qué features usa y por qué

| Feature de Go | Papel en este caso |
|--|--|
| Pool de N goroutines + channel | el mismo código para los dos tipos de tarea |
| `time.Sleep` | simula una llamada de red: la goroutine se aparca y no consume CPU |
| SHA-256 encadenado | CPU pura, calibrada a ≈ 10 ms por tarea |
| `runtime.GOMAXPROCS` | el nº de núcleos disponibles: el tope útil para CPU |

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ Para **dimensionar** cualquier pool: primero clasifica tu carga.
- ✅ E/S: sube N hasta que el recurso remoto (o tu semáforo) sea el límite.
- ❌ No hay un N universal: mide con tu carga real.
- ❌ Mezcla de E/S y CPU: separa en dos pools (uno por naturaleza) para no mezclar límites.

## Cómo ejecutarlo

```bash
go run .
go test -race -v .
```

## Qué observar (salida real, Apple M1, Go 1.26.5)

```
GOMAXPROCS=8 | 256 tareas de ≈10ms | rendimiento en tareas/s (mejor de 3; más es mejor)

workers     E/S (espera)  CPU (cálculo)
1                     92            131
2                    183            254
4                    367            469
8                    737            631
16                  1484            624
64                  5927            613
256                22897            630
```

## Errores típicos

- Dimensionar un pool de CPU con «muchas goroutines, que son baratas»: no aceleran, solo compiten (y empeoran la latencia).
- Dimensionar un pool de E/S con `NumCPU`: desperdicias el solape de esperas (aquí 8 workers dan 737/s frente a 22 897/s con 256).
- Olvidar que los núcleos del M1 no son iguales (4 de rendimiento + 4 de eficiencia): el CPU escala menos que `GOMAXPROCS`.
- Subir N sin límite hacia un servicio remoto: lo tumbas (concepto 5).

## Profundiza en el curso

- [CPU-bound frente a E/S-bound](../../../01-fundamentos/02-cpu-vs-io-bound)
- [tres cargas, tres techos](../../../13-rendimiento/01-escalabilidad)
- [worker pool](../../../08-patrones/01-worker-pool)
- [por qué 1 000 esperas no cuestan 1 000 hilos](../../../14-internals/02-syscalls-y-netpoller)
- [`GOMAXPROCS`](../../../02-goroutines/04-runtime-info)

## Prueba tú

1. Mezcla: 50 % E/S y 50 % CPU en el mismo pool; luego sepáralos en dos pools.
2. Pon `GOMAXPROCS=2` y repite.
3. Sustituye `Sleep` por una espera real (un `http.Get` a un servidor local) y compara.
