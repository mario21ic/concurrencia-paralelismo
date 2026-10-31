# Ejercicio 1: `select` básico

## Teoría

`select` es a los channels lo que `switch` a los valores, pero **espera**: bloquea hasta que algún caso pueda proceder y ejecuta exactamente uno.

- Si dos o más casos están listos, la elección es **pseudoaleatoria uniforme**. Esto evita inanición sistemática pero también significa que **no hay prioridad**.
- Para simular prioridad se usan **dos `select` anidados**: el primero con `default` sondea el channel prioritario; solo si está vacío se pasa al segundo `select` que acepta cualquiera.
- Limitación: la prioridad se evalúa en el instante del sondeo, no es una garantía global.

## Qué hace el programa

1. Tres channels con latencias 300/100/200 ms: gana el de 100 ms.
2. Dos channels siempre listos, 100 000 selecciones: cuenta cuántas veces gana cada uno.
3. Dos colas, una de alta y otra de baja prioridad, ambas con 3 mensajes.

```bash
go run .
```

## Resultado de referencia

```
primero llegó B
ambos listos, 100000 veces: x=49720 y=50280
con prioridad: alta1 alta2 alta3 baja1 baja2 baja3
```

## Conclusiones

- El reparto es ~50/50: no se puede asumir ningún orden entre casos listos.
- Sin el patrón de doble `select`, `alta` y `baja` se mezclarían.
- `select` espera a **cualquier** evento de varios: es la base de timeouts, cancelación y multiplexación.

## Ejercicios propuestos

1. Cambia a tres canales siempre listos y comprueba que el reparto sigue siendo uniforme (~33 %).
2. Quita el `default` del primer `select` de la parte 3: ¿qué cambia en la salida?
3. ¿Qué pasa con `select {}` en `main`? Pruébalo (y mira el mensaje del runtime).
