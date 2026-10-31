// Ejercicio 1: channels sin buffer vs con buffer.
package main

import (
	"fmt"
	"time"
)

func sinBuffer() {
	fmt.Println("== sin buffer: el envío espera al receptor (rendezvous) ==")
	ch := make(chan string)
	ini := time.Now()

	go func() {
		time.Sleep(200 * time.Millisecond) // el receptor tarda en llegar
		fmt.Printf("[%v] receptor listo\n", time.Since(ini).Round(10*time.Millisecond))
		fmt.Println("recibido:", <-ch)
	}()

	ch <- "hola" // bloquea hasta que el receptor reciba
	fmt.Printf("[%v] envío completado\n", time.Since(ini).Round(10*time.Millisecond))
	time.Sleep(10 * time.Millisecond)
}

func conBuffer() {
	fmt.Println("\n== con buffer (cap=3): el envío solo bloquea si está lleno ==")
	ch := make(chan int, 3)
	fmt.Printf("len=%d cap=%d\n", len(ch), cap(ch))

	for i := 1; ; i++ {
		select {
		case ch <- i:
			fmt.Printf("enviado %d  -> len=%d cap=%d\n", i, len(ch), cap(ch))
		default:
			fmt.Printf("enviar %d bloquearía: buffer lleno\n", i)
			goto vaciar
		}
	}
vaciar:
	for len(ch) > 0 {
		fmt.Println("recibido", <-ch) // FIFO
	}
}

func main() {
	sinBuffer()
	conBuffer()
}
