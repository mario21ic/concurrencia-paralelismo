// Ejercicio 1: select básico, elección aleatoria y prioridad.
package main

import (
	"fmt"
	"time"
)

func tras(d time.Duration, s string) <-chan string {
	ch := make(chan string, 1)
	go func() {
		time.Sleep(d)
		ch <- s
	}()
	return ch
}

func main() {
	// 1) select espera hasta que ALGÚN caso esté listo y ejecuta solo uno.
	a, b, c := tras(300*time.Millisecond, "A"), tras(100*time.Millisecond, "B"), tras(200*time.Millisecond, "C")
	select {
	case v := <-a:
		fmt.Println("primero llegó", v)
	case v := <-b:
		fmt.Println("primero llegó", v)
	case v := <-c:
		fmt.Println("primero llegó", v)
	}

	// 2) Si varios están listos, Go elige UNO AL AZAR (uniforme).
	x, y := make(chan int, 1), make(chan int, 1)
	var nx, ny int
	for range 100_000 {
		x <- 1
		y <- 1
		select {
		case <-x:
			nx++
			<-y // vaciar el otro
		case <-y:
			ny++
			<-x
		}
	}
	fmt.Printf("ambos listos, 100000 veces: x=%d y=%d\n", nx, ny)

	// 3) Prioridad: select no la tiene, pero se logra con dos selects.
	alta, baja := make(chan string, 5), make(chan string, 5)
	for i := 1; i <= 3; i++ {
		alta <- fmt.Sprintf("alta%d", i)
		baja <- fmt.Sprintf("baja%d", i)
	}
	fmt.Print("con prioridad: ")
	for range 6 {
		select {
		case v := <-alta: // primero intenta la de alta prioridad
			fmt.Print(v, " ")
		default:
			select { // si no hay, acepta cualquiera
			case v := <-alta:
				fmt.Print(v, " ")
			case v := <-baja:
				fmt.Print(v, " ")
			}
		}
	}
	fmt.Println()
}
