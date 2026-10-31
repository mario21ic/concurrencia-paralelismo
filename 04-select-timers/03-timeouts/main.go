// Ejercicio 3: timeouts con time.After y time.NewTimer.
package main

import (
	"fmt"
	"time"
)

func lento(d time.Duration) <-chan string {
	ch := make(chan string, 1) // buffer 1: la goroutine no se queda colgada si hubo timeout
	go func() {
		time.Sleep(d)
		ch <- "listo"
	}()
	return ch
}

// conTimeout limita UNA operación.
func conTimeout(op <-chan string, max time.Duration) (string, error) {
	select {
	case v := <-op:
		return v, nil
	case <-time.After(max):
		return "", fmt.Errorf("timeout tras %v", max)
	}
}

func main() {
	fmt.Println("== timeout de una operación ==")
	fmt.Println(conTimeout(lento(50*time.Millisecond), 200*time.Millisecond))
	fmt.Println(conTimeout(lento(500*time.Millisecond), 100*time.Millisecond))

	// Timeout por mensaje (se reinicia en cada iteración) vs. plazo global.
	datos := make(chan int)
	go func() {
		for i := 1; i <= 10; i++ {
			time.Sleep(60 * time.Millisecond)
			datos <- i
		}
		close(datos)
	}()

	fmt.Println("\n== plazo GLOBAL de 250ms (un solo timer) ==")
	plazo := time.NewTimer(250 * time.Millisecond)
	defer plazo.Stop()
	n := 0
bucle:
	for {
		select {
		case v, ok := <-datos:
			if !ok {
				break bucle
			}
			n = v
		case <-plazo.C:
			fmt.Println("plazo agotado")
			break bucle
		}
	}
	fmt.Println("mensajes recibidos antes del plazo:", n)

	fmt.Println("\n== inactividad: se reinicia con cada mensaje ==")
	inact := make(chan int)
	go func() {
		for i := 1; i <= 3; i++ {
			time.Sleep(50 * time.Millisecond)
			inact <- i
		}
		time.Sleep(time.Second) // silencio
	}()
	t := time.NewTimer(120 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case v := <-inact:
			fmt.Println("mensaje", v)
			t.Reset(120 * time.Millisecond) // Go 1.23+: Reset es seguro sin drenar
		case <-t.C:
			fmt.Println("sin actividad durante 120ms: fin")
			return
		}
	}
}
