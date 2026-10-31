# Ejercicio 2: channels direccionales

## Teoría

| Tipo | Permite | Prohíbe |
|--|--|--|
| `chan T` | enviar, recibir, cerrar | |
| `chan<- T` | enviar, cerrar | recibir |
| `<-chan T` | recibir | enviar, **cerrar** |

- La conversión de `chan T` a `chan<- T` o `<-chan T` es **implícita**; la inversa no existe.
- Las restricciones se verifican en **compilación**.
- En una firma, indica la intención: `func productor(out chan<- int)`, `func consumidor(in <-chan int)`.
- Una función que *crea* el channel y lanza la goroutine productora debe devolver `<-chan T`. Así el llamador **no puede** enviar ni cerrar por error, y la propiedad de cierre queda clara.
- Regla de lectura: la flecha apunta hacia dónde fluyen los datos respecto a la palabra `chan`.

## Qué hace el programa

Un productor (`chan<-`) envía 1..10 y cierra; un consumidor (`<-chan`) suma. Además una función `contar` devuelve `<-chan int`. Las líneas que no compilan están comentadas dentro de las funciones: descoméntalas para ver el error.

```bash
go run .
```

## Resultado de referencia

```
suma 1..10 = 55
contar: 0
contar: 1
contar: 2
```

Errores de compilación al descomentar:

```
invalid operation: cannot receive from send-only channel out
invalid operation: cannot send to receive-only channel in
invalid operation: cannot close receive-only channel in
```

## Conclusiones

- Los tipos direccionales convierten errores de concurrencia en errores de compilación.
- El cierre pertenece al emisor; con `<-chan` el receptor ni siquiera puede intentarlo.

## Ejercicios propuestos

1. Descomenta cada línea prohibida y anota el mensaje exacto del compilador.
2. Escribe `func duplicar(in <-chan int) <-chan int` que lea de `in` y emita el doble.
3. Intenta asignar un `<-chan int` a una variable `chan int`. ¿Compila?
