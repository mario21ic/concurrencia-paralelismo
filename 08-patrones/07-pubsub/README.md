# Ejercicio 7: publicador/suscriptor (broadcast)

## Teoría

En pub/sub **cada mensaje llega a todos los suscriptores** (a diferencia del fan-out, donde cada valor lo toma uno). Es el patrón para eventos, notificaciones, invalidación de cachés y *hot reload*.

```
                ┌─► suscriptor A (buffer 100)
publicador ─────┼─► suscriptor B (buffer 2)   ← lento
                └─► suscriptor C (efímero)
```

### El problema central: el suscriptor lento

Si el publicador envía con bloqueo, **un solo suscriptor lento frena a todos**. Políticas:

| Política | Efecto |
|--|--|
| **Descartar** (la implementada) | `select` con `default`: el mensaje se pierde *para ese suscriptor* y se cuenta |
| Bloquear | consistencia total, pero un lento frena al sistema |
| Bloquear con plazo | espera un tiempo y luego descarta o expulsa al suscriptor |
| Cola ilimitada por suscriptor | sin pérdidas, pero memoria sin tope |
| Expulsar al lento | cerrar su suscripción |

La elección es de **negocio**: ¿es peor perder un evento o retrasar a todos?

### Seguridad ante altas y bajas

El riesgo clásico es `send on closed channel`. Solución: el broker es el **único que cierra** los channels, y lo hace bajo `Lock`; `Publicar` toma `RLock` y como los envíos no bloquean, la sección es corta. Así es imposible cerrar mientras alguien envía.

Otras decisiones:
- `Cancelar()` es idempotente; `Cerrar()` también.
- `Suscribir(ctx, …)` se da de baja sola al terminar el contexto, vía `context.AfterFunc` (que se desregistra si cancelas antes).
- Tras `Cerrar()`, suscribir devuelve un channel ya cerrado y publicar entrega a 0.

## Qué hace el programa

Un suscriptor rápido (buffer 100), uno lento (buffer 2, 25 ms por mensaje) y uno efímero cuyo contexto se cancela a mitad. Se publican 20 mensajes cada 5 ms.

```bash
go run .
go test -race -count=3 .
```

## Resultado de referencia

```
suscriptores iniciales: 3
tras cancelar el contexto del efímero: 2 suscriptores

rápido:  recibió 20, descartó 0
lento:   recibió  7, descartó 13  <- no frenó a nadie
efímero: recibió  8, descartó 0  <- se dio de baja
publicar tras Cerrar entrega a: 0
```

## Conclusiones

- El rápido recibe los 20; el lento recibe 7 y se pierden 13, pero **el publicador no se frena** y el rápido no se ve afectado.
- Dar de baja por contexto funciona sin fugas ni panics.
- El test de concurrencia (4 publicadores + altas/bajas continuas, con `-race`) valida que no hay envíos a channels cerrados.

## Tests

Todos reciben todo, el lento descarta (98 de 100 con buffer 2) sin bloquear, `Cancelar` idempotente, baja por contexto, suscribir tras `Cerrar`, y estrés concurrente.

## Ejercicios propuestos

1. Añade **temas** (`Suscribir("pedidos")`) con un mapa de brokers o de suscriptores por tema.
2. Implementa la política "bloquear con plazo" y compárala.
3. Expulsa al suscriptor tras N descartes consecutivos.
4. Añade mensajes **retenidos** (el último valor se entrega a quien se suscribe).
