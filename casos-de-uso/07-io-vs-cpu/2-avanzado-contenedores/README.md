# Caso 7.2 — Salir del proceso: paralelismo con contenedores  ·  `avanzado`

> **Problema:** una sola máquina (o un solo proceso) ya no basta, o quieres aislar fallos y escalar «de verdad». ¿Cómo se reparte el trabajo CPU entre varios procesos y qué cambia en el diseño?

Este caso no tiene código propio: **es el [proyecto final (`proyecto-final/`)](../../../proyecto-final)**, un planificador de trabajos con un coordinador y N workers en contenedores. Aquí está el porqué y el cuándo.

## Qué cambia al salir del proceso

| En un proceso | Entre procesos / contenedores |
|--|--|
| `sync.Mutex`, `channel`, `atomic` | **no existen**: se sustituyen por mensajes de red (HTTP/gRPC), colas o un almacén compartido |
| una goroutine que muere mata el proceso | un worker puede **morir sin que el sistema caiga** (el trabajo se reasigna) |
| la entrega es «exactamente una vez» (memoria) | **«al menos una vez»**: los trabajos deben ser **idempotentes** |
| `context` cancela | hace falta un *protocolo*: **arriendos con caducidad** y latidos |
| el planificador de Go reparte entre núcleos | tú repartes entre contenedores; el orquestador entre máquinas |

## Las piezas del curso que se reutilizan (las mismas, a mayor escala)

- **Cola con prioridades y envejecimiento** (inanición, módulo 15) · **reintentos con backoff y jitter** (caso 4.2) · **contrapresión** (503) y **cuotas** (429, caso 5.3) · **apagado ordenado con gracia** (caso 4.3) · **long polling** (goroutines aparcadas, módulo 14) · **`GOMAXPROCS` consciente de cgroups** (Go 1.25+).

## Resultado medido (Docker Compose, cada worker limitado a 1 CPU)

300 trabajos de CPU (≈ 85 ms cada uno):

```
workers   total      jobs/s   speedup   eficiencia
1         26.1 s       11      1.00x      100 %
2         13.3 s       23      2.09x      105 %
4          6.6 s       46      4.18x      105 %
8          3.7 s       81      7.36x       92 %
```

Repartir en **procesos** da un speedup casi lineal porque cada worker tiene su propia CPU y su propia memoria (no hay contención de cerrojos entre ellos); el único punto compartido es el coordinador, que **no** era el cuello de botella.

## ¿Cuándo salir del proceso?

- ✅ La carga **CPU** supera los núcleos de una máquina y es divisible en trabajos independientes.
- ✅ Necesitas **tolerar fallos** (un worker puede morir) o **desplegar sin parar** el servicio.
- ✅ Quieres **escalar por separado** la parte pesada (workers) de la ligera (API).
- ❌ Si 1 máquina te sobra: es más simple (sin red, sin reintentos, sin idempotencia, sin despliegue).
- ❌ Si el trabajo es tan pequeño que la red cuesta más que él (agrúpalo en lotes: módulo 13).

## Pruébalo

**Versión pequeña** (solo workers y un cliente, ≈ 1 min): [caso 7.3](../3-avanzado-compose-escalado), con `./scripts/escalar.sh`. **Versión completa** (cola, arriendos, tolerancia a fallos):

```bash
cd ../../../proyecto-final
docker compose up -d --build
./scripts/escalado.sh 300 600000 "1 2 4 8"      # mide el speedup en tu máquina
docker compose down
```

## Profundiza

- [`proyecto-final/README.md`](../../../proyecto-final/README.md) — decisiones de diseño, API y todos los experimentos (tolerancia a fallos, apagado, contrapresión).
- [13-rendimiento/01-escalabilidad](../../../13-rendimiento/01-escalabilidad) — por qué el techo de una máquina depende del recurso limitante.
- [01-fundamentos/05-amdahl](../../../01-fundamentos/05-amdahl) — la parte secuencial limita el speedup.
- [15-diseno-avanzado/10-http-backpressure](../../../15-diseno-avanzado/10-http-backpressure) — proteger el coordinador bajo sobrecarga.
