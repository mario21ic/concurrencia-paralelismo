// Ejercicio 8: generadores con channels.
//
// Un generador devuelve <-chan T y produce valores en una goroutine.
// Problema: si el consumidor se detiene antes, la goroutine queda
// bloqueada enviando para siempre (fuga). Solución: canal `done`.
package main

import (
	"fmt"
	"runtime"
	"time"
)

// fibIngenuo: no se puede detener; fuga si el consumidor abandona.
func fibIngenuo() <-chan int {
	ch := make(chan int)
	go func() {
		defer close(ch)
		a, b := 0, 1
		for {
			ch <- a // se bloquea aquí si nadie lee
			a, b = b, a+b
		}
	}()
	return ch
}

// fib: se detiene cuando done se cierra.
func fib(done <-chan struct{}) <-chan int {
	ch := make(chan int)
	go func() {
		defer close(ch)
		a, b := 0, 1
		for {
			select {
			case ch <- a:
				a, b = b, a+b
			case <-done:
				return
			}
		}
	}()
	return ch
}

func primeros(ch <-chan int, n int) []int {
	var out []int
	for v := range ch {
		out = append(out, v)
		if len(out) == n {
			break
		}
	}
	return out
}

func main() {
	base := runtime.NumGoroutine()

	fmt.Println("ingenuo:", primeros(fibIngenuo(), 10))
	time.Sleep(20 * time.Millisecond)
	fmt.Println("goroutines fugadas tras el ingenuo:", runtime.NumGoroutine()-base)

	done := make(chan struct{})
	fmt.Println("con done:", primeros(fib(done), 10))
	close(done)
	time.Sleep(20 * time.Millisecond)
	fmt.Println("goroutines fugadas en total:", runtime.NumGoroutine()-base, "(solo queda la del ingenuo)")
}
