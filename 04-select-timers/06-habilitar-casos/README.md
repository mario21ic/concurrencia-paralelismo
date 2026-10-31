# Ejercicio 6: habilitar y deshabilitar casos con `nil`

## Teoría

En un `select`, un caso cuyo channel es `nil` **no se puede elegir nunca**. Eso permite *activar y desactivar* casos en tiempo de ejecución, calculando en cada vuelta qué variables apuntan a un channel real y cuáles a `nil`.

Patrón: **buffer intermedio acotado** (un `select` de dos direcciones):

| Condición | Caso de recepción (`entrada`) | Caso de envío (`salida`) |
|--|--|--|
| cola vacía | activo | **apagado** (no hay nada que enviar) |
| cola con datos, no llena | activo | activo |
| cola llena | **apagado** (contrapresión) | activo |
| entrada cerrada | apagado para siempre (`nil`) | activo mientras haya datos |

El bucle termina cuando `entrada` es `nil` y la cola está vacía.

Es la forma sin locks de modelar una cola entre un productor rápido y un consumidor lento con **contrapresión** (*backpressure*): al llenarse la cola, el productor se bloquea.

Trampa: el valor a enviar en un `case ch <- valor` se evalúa **al entrar al `select`**, aunque el caso esté apagado; por eso se calcula `siguiente` solo cuando hay datos.

## Qué hace el programa

Un productor emite 1..8 sin pausa; un consumidor tarda 30 ms por elemento; la cola intermedia admite 3. Se imprime cada encolado y consumo.

```bash
go run .
```

## Resultado de referencia

```
   encolado 1 (en cola: 1)
   encolado 2 (en cola: 2)
   encolado 3 (en cola: 2)
   encolado 4 (en cola: 3)
consumido 1
consumido 2
   encolado 5 (en cola: 3)
...
consumido 8
```

(Los números exactos de cola varían por el reparto de goroutines.)

## Conclusiones

- La cola nunca supera 3: el productor se frena cuando se llena.
- No hay locks ni variables compartidas: toda la lógica vive en una goroutine con `select`.
- El mismo mecanismo se usó en el módulo 3 para fusionar channels.

## Ejercicios propuestos

1. Cambia `max` a 1 y a 100; relaciona el tamaño con la latencia observada.
2. Añade un caso `<-done` para cancelar todo el buffer.
3. Haz que el buffer descarte el más antiguo en vez de bloquear al productor.
