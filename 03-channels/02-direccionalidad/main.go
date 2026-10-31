// Ejercicio 2: channels direccionales.
//
//	chan T    bidireccional
//	chan<- T  solo envío
//	<-chan T  solo recepción
//
// Un chan T se convierte implícitamente a cualquiera de los dos, pero no al revés.
package main

import "fmt"

// productor solo puede ENVIAR y es quien cierra.
func productor(out chan<- int, n int) {
	defer close(out)
	for i := 1; i <= n; i++ {
		out <- i
	}
	// <-out        // error de compilación: receive from send-only channel
}

// consumidor solo puede RECIBIR.
func consumidor(in <-chan int) int {
	suma := 0
	for v := range in {
		suma += v
	}
	// in <- 1      // error de compilación: send to receive-only channel
	// close(in)    // error de compilación: close of receive-only channel
	return suma
}

// Una función puede devolver <-chan: el llamador solo puede leer
// y el cierre queda en manos de quien lo creó.
func contar(n int) <-chan int {
	ch := make(chan int)
	go func() {
		defer close(ch)
		for i := range n {
			ch <- i
		}
	}()
	return ch
}

func main() {
	ch := make(chan int) // bidireccional
	go productor(ch, 10) // se convierte a chan<- int
	fmt.Println("suma 1..10 =", consumidor(ch))

	for v := range contar(3) {
		fmt.Println("contar:", v)
	}
}
