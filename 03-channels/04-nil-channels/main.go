// Ejercicio 4: channels nil.
//
// Enviar o recibir de un channel nil bloquea para siempre. En un select eso
// significa "este caso nunca se elige", útil para desactivar casos.
package main

import (
	"fmt"
	"time"
)

func emitir(vals ...int) <-chan int {
	ch := make(chan int)
	go func() {
		defer close(ch)
		for _, v := range vals {
			time.Sleep(10 * time.Millisecond)
			ch <- v
		}
	}()
	return ch
}

// fusionar une dos channels. Cuando uno se cierra se pone a nil para que
// select deje de elegirlo (si no, giraría recibiendo ceros sin parar).
func fusionar(a, b <-chan int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for a != nil || b != nil {
			select {
			case v, ok := <-a:
				if !ok {
					a = nil
					continue
				}
				out <- v
			case v, ok := <-b:
				if !ok {
					b = nil
					continue
				}
				out <- v
			}
		}
	}()
	return out
}

func main() {
	var nilCh chan int
	fmt.Println("nilCh == nil:", nilCh == nil, "len:", len(nilCh), "cap:", cap(nilCh))

	select {
	case v := <-nilCh:
		fmt.Println("imposible", v)
	case <-time.After(100 * time.Millisecond):
		fmt.Println("recibir de nil: bloqueado (salimos por timeout)")
	}

	for v := range fusionar(emitir(1, 2, 3), emitir(100, 200)) {
		fmt.Println("fusionado:", v)
	}
}
