// Caso 3.1 (básico): un productor entrega datos a un consumidor por un channel.
//
// Problema: una goroutine genera valores y otra los procesa a su ritmo. Usamos un channel en vez de
// una variable compartida: el dato SE ENTREGA (transfiere la propiedad), no se comparte.
//
// Reglas de oro que este caso fija:
//
//  1. Quien ENVÍA cierra (nunca el receptor).
//
//  2. `for v := range ch` termina solo cuando el channel se cierra.
//
//  3. El buffer es un amortiguador: desacopla el ritmo del productor del del consumidor.
//
//     go run .
package main

import (
	"fmt"
	"time"
)

// Productor genera los cuadrados de 1..n y cierra el channel al terminar.
func Productor(n int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out) // regla 1: el productor cierra
		for i := 1; i <= n; i++ {
			out <- i * i
		}
	}()
	return out
}

// Sumar consume hasta que el productor cierre (regla 2).
func Sumar(in <-chan int) (suma int) {
	for v := range in {
		suma += v
	}
	return
}

// Adelanto mide el efecto del buffer: el productor intenta enviar 10 valores mientras el consumidor
// tarda 50 ms en cada uno. A los 25 ms (el consumidor aún trabaja en el 1.º), ¿cuántos valores han
// salido ya del productor?
//   - Sin buffer: el productor solo avanza al ritmo del consumidor.
//   - Con buffer N: el productor se adelanta hasta llenarlo.
func Adelanto(buffer int) (enviados int) {
	ch := make(chan int, buffer)
	cuenta := make(chan int, 16)
	go func() {
		defer close(ch)
		for i := range 10 {
			ch <- i
			cuenta <- i
		}
	}()
	fin := make(chan struct{})
	go func() {
		defer close(fin)
		for range ch {
			time.Sleep(50 * time.Millisecond)
		}
	}()
	time.Sleep(25 * time.Millisecond)
	enviados = len(cuenta)
	<-fin // deja que todo termine: ninguna goroutine debe sobrevivir a la función
	return enviados
}

func main() {
	fmt.Println("suma de cuadrados 1..10 =", Sumar(Productor(10)))
	for _, b := range []int{0, 1, 4} {
		fmt.Printf("buffer=%d → a los 25 ms el productor ya envió %d de 10\n", b, Adelanto(b))
	}
}
