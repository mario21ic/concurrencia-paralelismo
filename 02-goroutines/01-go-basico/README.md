# Ejercicio 1: la palabra clave `go`

## Teoría

- `go f(a, b)` crea una goroutine. Los argumentos se **evalúan inmediatamente** en la goroutine que llama; la ejecución de `f` ocurre más tarde y en un orden no determinista.
- No hay forma de "esperar" implícitamente: `main` retorna → el proceso termina, aunque queden goroutines vivas.
- El valor de retorno de la función lanzada se **pierde**. Para recuperarlo: channel o variable protegida.
- `time.Sleep` "arregla" el ejemplo solo por casualidad: no hay garantía de que 100 ms sean suficientes, y retrasa el programa aunque terminen antes.
- La forma correcta de esperar es `sync.WaitGroup` (o un channel).

## Qué hace el programa

```bash
go run . -modo=sin-espera   # main termina antes que las goroutines
go run . -modo=sleep        # funciona, pero es un anti-patrón
go run . -modo=waitgroup    # correcto
go run . -modo=retorno      # el retorno se pierde; se recupera con un channel
```

## Resultado de referencia

```
-modo=sin-espera
hola desde la goroutine 1      <- imprime 0, 1, 2 o 3 mensajes según la ejecución
main termina

-modo=waitgroup
hola desde la goroutine 2
hola desde la goroutine 1      <- siempre las 3, en orden no determinista
hola desde la goroutine 0
main termina
```

## Conclusiones

- El **orden** de ejecución de goroutines no está definido. No lo asumas jamás.
- Ejecuta `sin-espera` varias veces: el resultado cambia.
- Sleep ≠ sincronización.

## Ejercicios propuestos

1. Ejecuta `-modo=sin-espera` 20 veces y cuenta cuántos mensajes salen de media.
2. Cambia `saludar` para que devuelva un `string` y recoge los 3 resultados mediante un channel.
3. ¿Qué ocurre si lanzas la goroutine con `go func() {...}` pasando una variable que cambias justo después? (anticipo del ejercicio 2)
