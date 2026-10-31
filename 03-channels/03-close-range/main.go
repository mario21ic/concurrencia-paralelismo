// Ejercicio 3: close, range y el idioma v, ok := <-ch.
package main

import (
	"fmt"
	"sync"
	"time"
)

func main() {
	// 1) Recibir de un channel cerrado devuelve el valor cero y ok=false.
	ch := make(chan int, 2)
	ch <- 10
	ch <- 20
	close(ch)
	for range 4 {
		v, ok := <-ch
		fmt.Printf("v=%d ok=%v\n", v, ok) // primero drena el buffer, luego ceros
	}

	// 2) range termina solo cuando el channel se cierra.
	c2 := make(chan string)
	go func() {
		defer close(c2)
		for _, s := range []string{"a", "b", "c"} {
			c2 <- s
		}
	}()
	for s := range c2 {
		fmt.Println("range:", s)
	}

	// 3) close como señal de difusión: todos los receptores se despiertan.
	listo := make(chan struct{})
	var wg sync.WaitGroup
	for i := range 3 {
		wg.Go(func() {
			<-listo // bloquea hasta el close
			fmt.Println("worker", i, "arrancó")
		})
	}
	time.Sleep(50 * time.Millisecond)
	fmt.Println("soltando a todos...")
	close(listo) // un solo close libera a todos (un envío liberaría solo a uno)
	wg.Wait()
}
