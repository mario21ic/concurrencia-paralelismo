// Ejercicio 4: modelo CSP.
//
// El mismo contador implementado de dos formas:
//   - memoria compartida protegida con Mutex
//   - una goroutine "dueña" del estado; los demás le envían mensajes
//
// Y un ping-pong que muestra la sincronización de un channel sin buffer.
package main

import (
	"fmt"
	"sync"
	"time"
)

const (
	goroutines  = 100
	incrementos = 1000
)

// contadorMutex: comparte memoria y la protege con un lock.
func contadorMutex() int {
	var (
		mu sync.Mutex
		n  int
		wg sync.WaitGroup
	)
	for range goroutines {
		wg.Go(func() {
			for range incrementos {
				mu.Lock()
				n++
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	return n
}

// contadorCSP: solo una goroutine toca n; el resto comunica por channel.
func contadorCSP() int {
	inc := make(chan struct{})
	resultado := make(chan int)

	go func() { // dueña del estado
		n := 0
		for range inc {
			n++
		}
		resultado <- n
	}()

	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			for range incrementos {
				inc <- struct{}{}
			}
		})
	}
	wg.Wait()
	close(inc)
	return <-resultado
}

func pingPong() {
	ping, pong := make(chan int), make(chan int)
	go func() {
		for v := range ping {
			pong <- v + 1
		}
		close(pong)
	}()
	v := 0
	for range 5 {
		ping <- v
		v = <-pong
		fmt.Println("ping-pong, valor:", v)
	}
	close(ping)
}

func main() {
	esperado := goroutines * incrementos

	ini := time.Now()
	fmt.Printf("Mutex: %d (esperado %d) en %v\n", contadorMutex(), esperado, time.Since(ini).Round(time.Microsecond))

	ini = time.Now()
	fmt.Printf("CSP:   %d (esperado %d) en %v\n", contadorCSP(), esperado, time.Since(ini).Round(time.Microsecond))

	fmt.Println()
	pingPong()
}
