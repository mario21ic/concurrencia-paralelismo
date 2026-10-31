# Ejercicio 7: ping-pong

## Enunciado

Dos goroutines se pasan una pelota por dos channels sin buffer. Medir cuánto cuesta cada ida y vuelta.

## Teoría

```
main ──ping──►  jugador
main ◄──pong──  jugador
```

- Cada intercambio por channel sin buffer obliga a **despertar** a la otra goroutine y cambiar de contexto en el scheduler, no en el kernel.
- Se pasa un **puntero** a la misma estructura: solo una goroutine la toca en cada momento (la propiedad se *transfiere* con el mensaje). Es el modelo CSP: sin locks aunque el dato sea mutable.
- El jugador cierra `salida` al terminar su `range`, propagando el final.

## Cómo ejecutarlo

```bash
go run .               # 5 intercambios, con trazas
go run . -n 1000000    # medición
```

## Resultado de referencia

```
ping (golpe 1) ... ping (golpe 5)
golpes: 5 en 258µs

golpes: 1000000 en 266.59ms
por ida y vuelta: ~266ns
```

## Conclusiones

- Una ida y vuelta (2 sincronizaciones) cuesta ~266 ns de media en régimen: unos ~130 ns por traspaso. Es barato, pero **no gratis**: un mutex sin contención cuesta ~20 ns.
- Con pocas iteraciones el tiempo por vuelta es mucho mayor (51 µs con n=5) por arranque en frío: mide con muchas repeticiones.
- No comuniques por channel a cada elemento si puedes agrupar (batching).

## Ejercicios propuestos

1. Haz el ping-pong con `GOMAXPROCS=1` y compara. ¿Por qué cambia?
2. Añade un tercer jugador formando un anillo.
3. Usa channels con buffer 1: ¿cambia la latencia? ¿y la semántica?
