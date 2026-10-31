// Ejercicio 7: ping-pong entre dos goroutines.
//
// Se pasan una "pelota" por dos channels sin buffer. Cada ida y vuelta
// obliga a sincronizar ambas goroutines, lo que permite medir el costo de
// un cambio de contexto entre goroutines.
package main

import (
	"flag"
	"fmt"
	"time"
)

type pelota struct{ golpes int }

func jugador(nombre string, entrada <-chan *pelota, salida chan<- *pelota, verbose bool) {
	for p := range entrada {
		p.golpes++
		if verbose {
			fmt.Printf("%s (golpe %d)\n", nombre, p.golpes)
		}
		salida <- p
	}
	close(salida)
}

func main() {
	n := flag.Int("n", 5, "idas y vueltas")
	flag.Parse()
	verbose := *n <= 10

	ping, pong := make(chan *pelota), make(chan *pelota)
	go jugador("ping", ping, pong, verbose)

	p := &pelota{}
	ini := time.Now()
	for range *n {
		ping <- p
		p = <-pong
	}
	d := time.Since(ini)
	close(ping)

	fmt.Printf("golpes: %d en %v\n", p.golpes, d.Round(time.Microsecond))
	fmt.Printf("por ida y vuelta: ~%v\n", d/time.Duration(*n))
}
