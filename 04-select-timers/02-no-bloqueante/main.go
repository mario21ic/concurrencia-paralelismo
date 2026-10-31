// Ejercicio 2: select con default => operaciones no bloqueantes.
package main

import "fmt"

func tryRecv(ch <-chan int) (int, bool) {
	select {
	case v := <-ch:
		return v, true
	default:
		return 0, false
	}
}

func trySend(ch chan<- int, v int) bool {
	select {
	case ch <- v:
		return true
	default:
		return false
	}
}

func main() {
	ch := make(chan int, 2)

	_, ok := tryRecv(ch)
	fmt.Println("recibir de vacío:", ok)

	for i := 1; i <= 4; i++ {
		fmt.Printf("trySend(%d) = %v\n", i, trySend(ch, i))
	}

	v, ok := tryRecv(ch)
	fmt.Println("recibir:", v, ok)

	// Caso real: encolar eventos sin frenar al productor; si la cola
	// está llena se DESCARTA (load shedding) y se cuenta.
	cola := make(chan int, 5)
	descartados := 0
	for i := range 20 {
		select {
		case cola <- i:
		default:
			descartados++
		}
	}
	fmt.Printf("cola: %d aceptados, %d descartados\n", len(cola), descartados)

	// Anti-patrón: select{default} en bucle = espera activa (100% CPU).
	// for { select { case v := <-ch: ...; default: } }
}
