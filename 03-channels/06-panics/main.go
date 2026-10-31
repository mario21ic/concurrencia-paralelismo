// Ejercicio 6: panics típicos con channels.
//
// Se recuperan con recover() solo para mostrar el mensaje; en código real
// son bugs de diseño que no deberían ocurrir.
package main

import "fmt"

func intentar(nombre string, f func()) {
	defer func() {
		fmt.Printf("%-24s -> %v\n", nombre, recover())
	}()
	f()
}

func main() {
	intentar("enviar a cerrado", func() {
		ch := make(chan int, 1)
		close(ch)
		ch <- 1
	})

	intentar("cerrar dos veces", func() {
		ch := make(chan int)
		close(ch)
		close(ch)
	})

	intentar("cerrar nil", func() {
		var ch chan int
		close(ch)
	})

	// Esto NO es panic: recibir de cerrado es seguro.
	ch := make(chan int)
	close(ch)
	v, ok := <-ch
	fmt.Printf("%-24s -> v=%d ok=%v (sin panic)\n", "recibir de cerrado", v, ok)

	intentar("buffer negativo", func() {
		n := -1
		_ = make(chan int, n)
	})
}
