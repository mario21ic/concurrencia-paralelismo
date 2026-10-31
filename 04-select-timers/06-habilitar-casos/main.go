// Ejercicio 6: habilitar y deshabilitar casos de select con nil.
//
// Un buffer intermedio: recibe de `entrada` y entrega por `salida`.
// Se ENVÍA solo cuando hay algo pendiente y se RECIBE solo cuando hay sitio;
// el resto de casos se apaga poniendo el channel a nil.
package main

import (
	"fmt"
	"time"
)

func productor() <-chan int {
	ch := make(chan int)
	go func() {
		defer close(ch)
		for i := 1; i <= 8; i++ {
			ch <- i
		}
	}()
	return ch
}

func main() {
	entrada := productor()
	salida := make(chan int)

	// consumidor lento
	terminado := make(chan struct{})
	go func() {
		defer close(terminado)
		for v := range salida {
			fmt.Println("consumido", v)
			time.Sleep(30 * time.Millisecond)
		}
	}()

	const max = 3
	var cola []int
	for entrada != nil || len(cola) > 0 {
		var salidaActiva chan<- int // nil => caso de envío apagado
		var siguiente int
		if len(cola) > 0 {
			salidaActiva = salida
			siguiente = cola[0]
		}
		entradaActiva := entrada
		if len(cola) >= max {
			entradaActiva = nil // buffer lleno: no aceptar más
		}

		select {
		case v, ok := <-entradaActiva:
			if !ok {
				entrada = nil // cerrado: apagar para siempre
				continue
			}
			cola = append(cola, v)
			fmt.Printf("   encolado %d (en cola: %d)\n", v, len(cola))
		case salidaActiva <- siguiente:
			cola = cola[1:]
		}
	}
	close(salida)
	<-terminado
}
