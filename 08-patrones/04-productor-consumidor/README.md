# Ejercicio 4: productor–consumidor, contrapresión y batching

## Teoría

Productores generan trabajo y consumidores lo procesan; una **cola** los desacopla.

```
productores ──► [cola acotada] ──► consumidores
```

### Contrapresión (backpressure)

Si la cola es **acotada**, cuando se llena el productor se **bloquea** en el envío: el sistema se autorregula al ritmo del consumidor más lento. Si la cola es **ilimitada** (o enorme), el productor no se entera de que el consumidor no da abasto y la memoria crece hasta el desastre.

| Buffer | Efecto |
|--|--|
| 0 (sin buffer) | sincronización total; el productor espera al consumidor |
| N pequeño | absorbe ráfagas cortas, regula |
| enorme | oculta el problema; picos de memoria; latencia alta |

### Cierre con varios productores

Los productores **no** cierran la cola; lo hace un **coordinador** cuando todos terminan (`wgP.Wait(); close(cola)`). Los consumidores terminan al agotarse el `range`.

### Batching

Procesar de uno en uno puede ser caro (una escritura a base de datos por elemento). `Batch` agrupa en lotes de hasta `tam` elementos y los emite **cuando se llena el lote o pasa `espera`** desde su primer elemento, lo que ocurra antes: máximo rendimiento sin latencia ilimitada. Al cerrarse la entrada, se vacía el lote pendiente.

Detalles:
- El temporizador se arma solo al llegar el primer elemento del lote; mientras no hay lote, el canal del timer es `nil` y el caso queda apagado en el `select` (módulo 4).
- Se detiene el timer al vaciar por tamaño.

## Qué hace el programa

1. 3 productores rápidos × 20 elementos y 2 consumidores de 10 ms, con buffer 0, 5 y 100. Se mide cuándo terminan los productores, el total, la longitud máxima de la cola y el tiempo que los productores pasan bloqueados.
2. `Batch` con lotes de 4 o 50 ms.

```bash
go run .
go test -race -count=3 .
```

## Resultado de referencia

```
  buffer   productores acabaron        total    cola máx. productores bloqueados
       0                  291ms        301ms            0                852ms
       5                  271ms        301ms            5                732ms
     100                     0s        301ms           58                   0s

  [0s] lote: [1 2 3 4]
  [0s] lote: [5 6 7 8]
  [50ms] lote: [9 10]
  [120ms] lote: [11 12]
```

## Conclusiones

- **El tiempo total no cambia (301 ms)**: lo manda el consumidor (60 elementos × 10 ms / 2). El buffer no hace más rápido al sistema.
- Con buffer 0 o 5 los productores acaban casi al mismo tiempo que los consumidores: están **frenados** (contrapresión). Con buffer 100 acaban al instante y la cola llega a 58 elementos: **memoria** a cambio de nada.
- Los lotes `[9 10]` salen a los 50 ms por tiempo; `[11 12]` sale al cerrarse la entrada.

## Ejercicios propuestos

1. Añade métricas: elementos en cola vs. tiempo (¿se estabiliza?).
2. Si el consumidor falla, ¿qué hace el productor? Diseña una política (reintentar, descartar, parar).
3. Implementa `Batch` con `maxBytes` en lugar de número de elementos.
4. Añade un tercer consumidor y mide el cambio de tiempo total.
