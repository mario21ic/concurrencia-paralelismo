// Ejercicio 5: scheduler (G-M-P), cooperación y preemption.
//
//	go run . -modo=preemption
//	go run . -modo=gosched
//	GODEBUG=schedtrace=200 go run . -modo=carga
package main

import (
	"flag"
	"fmt"
	"runtime"
	"sync"
	"time"
)

// preemption: con un solo P, un bucle infinito sin llamadas a funciones
// NO bloquea al resto. Desde Go 1.14 hay preemption asíncrona (señales).
func preemption() {
	runtime.GOMAXPROCS(1)
	go func() {
		for { // acapara el único P
		}
	}()
	time.Sleep(10 * time.Millisecond)
	fmt.Println("main siguió ejecutando: el bucle fue expropiado")
}

// gosched: cesión voluntaria. Con 1 P las goroutines se alternan
// de forma determinista cuando llaman a runtime.Gosched().
func gosched() {
	runtime.GOMAXPROCS(1)
	var wg sync.WaitGroup
	for _, nombre := range []string{"A", "B", "C"} {
		wg.Go(func() {
			for i := 1; i <= 3; i++ {
				fmt.Printf("%s%d ", nombre, i)
				runtime.Gosched()
			}
		})
	}
	wg.Wait()
	fmt.Println()
}

// carga: muchas goroutines de CPU; úsalo con GODEBUG=schedtrace
// para ver los P ocupados y las colas locales/global.
func carga() {
	var wg sync.WaitGroup
	for range 64 {
		wg.Go(func() {
			x := 0
			for i := 0; i < 300_000_000; i++ {
				x += i
			}
			_ = x
		})
	}
	wg.Wait()
}

func main() {
	modo := flag.String("modo", "preemption", "preemption|gosched|carga")
	flag.Parse()
	switch *modo {
	case "preemption":
		preemption()
	case "gosched":
		gosched()
	case "carga":
		carga()
	default:
		flag.Usage()
	}
}
