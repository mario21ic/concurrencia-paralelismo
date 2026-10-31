// Ejercicio 5: deadlocks.
//
// Cada modo provoca un deadlock distinto. Los totales terminan con:
//
//	fatal error: all goroutines are asleep - deadlock!
//
// Ejecuta cada uno y explica por qué. El modo "parcial" muestra el caso que
// el detector NO atrapa: una fuga de goroutines.
package main

import (
	"flag"
	"fmt"
	"runtime"
	"sync"
	"time"
)

func main() {
	modo := flag.String("modo", "envio-sin-receptor",
		"envio-sin-receptor|recepcion-sin-emisor|range-sin-close|waitgroup|mutex|parcial")
	flag.Parse()

	switch *modo {
	case "envio-sin-receptor":
		ch := make(chan int)
		ch <- 1 // nadie recibirá jamás
		fmt.Println("no se llega aquí")

	case "recepcion-sin-emisor":
		ch := make(chan int)
		fmt.Println(<-ch) // nadie enviará jamás

	case "range-sin-close":
		ch := make(chan int)
		go func() {
			for i := range 3 {
				ch <- i
			}
			// falta close(ch)
		}()
		for v := range ch { // tras el 3er valor espera para siempre
			fmt.Println("recibido", v)
		}

	case "waitgroup":
		var wg sync.WaitGroup
		wg.Add(2)
		wg.Go(func() {}) // Add(1)+Done: queda un contador en 2+1-1 = 2
		wg.Wait()

	case "mutex":
		var mu sync.Mutex
		mu.Lock()
		mu.Lock() // sync.Mutex no es reentrante

	case "parcial":
		// El detector solo salta si TODAS las goroutines están bloqueadas.
		// Aquí main sigue vivo, así que no hay error: solo una fuga.
		for range 5 {
			go func() { <-make(chan int) }()
		}
		time.Sleep(50 * time.Millisecond)
		fmt.Println("goroutines vivas (fugadas):", runtime.NumGoroutine()-1)
	default:
		flag.Usage()
	}
}
