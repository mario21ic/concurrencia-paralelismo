// Ejercicio 1: la palabra clave go y el fin del programa.
//
//	go run . -modo=sin-espera   # main termina antes: casi nunca imprime
//	go run . -modo=sleep        # "funciona", pero es frágil (anti-patrón)
//	go run . -modo=waitgroup    # forma correcta
//	go run . -modo=retorno      # go descarta el valor de retorno
package main

import (
	"flag"
	"fmt"
	"sync"
	"time"
)

func saludar(id int) {
	fmt.Printf("hola desde la goroutine %d\n", id)
}

func suma(a, b int) int {
	fmt.Println("suma ejecutada en otra goroutine")
	return a + b
}

func main() {
	modo := flag.String("modo", "waitgroup", "sin-espera|sleep|waitgroup|retorno")
	flag.Parse()

	switch *modo {
	case "sin-espera":
		for i := range 3 {
			go saludar(i)
		}
		fmt.Println("main termina")
		// main retorna => el proceso muere, con goroutines pendientes.

	case "sleep":
		for i := range 3 {
			go saludar(i)
		}
		time.Sleep(100 * time.Millisecond) // ¿por qué 100ms? ¿y si la máquina está cargada?
		fmt.Println("main termina")

	case "waitgroup":
		var wg sync.WaitGroup
		for i := range 3 {
			wg.Go(func() { saludar(i) })
		}
		wg.Wait()
		fmt.Println("main termina")

	case "retorno":
		go suma(1, 2) // el resultado se pierde: para obtenerlo hay que usar channel o variable sincronizada
		time.Sleep(50 * time.Millisecond)

		res := make(chan int)
		go func() { res <- suma(3, 4) }()
		fmt.Println("resultado vía channel:", <-res)
	default:
		flag.Usage()
	}
}
