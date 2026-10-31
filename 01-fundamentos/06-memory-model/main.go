// Ejercicio 6: memory model y happens-before.
//
//	go run . -modo=race     # data race: resultado incorrecto
//	go run -race . -modo=race   # el detector lo reporta
//	go run . -modo=mutex|atomic|channel   # versiones correctas
//	go run . -modo=publicar-mal|publicar-bien
package main

import (
	"flag"
	"fmt"
	"sync"
	"sync/atomic"
)

const (
	goroutines  = 100
	incrementos = 10_000
)

func race() int {
	n := 0
	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			for range incrementos {
				n++ // lectura-modificación-escritura NO atómica
			}
		})
	}
	wg.Wait()
	return n
}

func conMutex() int {
	var mu sync.Mutex
	n := 0
	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			for range incrementos {
				mu.Lock() // Unlock happens-before el siguiente Lock
				n++
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	return n
}

func conAtomic() int {
	var n atomic.Int64
	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			for range incrementos {
				n.Add(1)
			}
		})
	}
	wg.Wait()
	return int(n.Load())
}

func conChannel() int {
	ch := make(chan int, goroutines)
	for range goroutines {
		go func() {
			local := 0
			for range incrementos {
				local++
			}
			ch <- local // el envío happens-before la recepción
		}()
	}
	total := 0
	for range goroutines {
		total += <-ch
	}
	return total
}

// publicarMal: sin sincronización nada garantiza que el lector vea `dato`
// aunque ya vea `listo` (el compilador o la CPU pueden reordenar, y el
// bucle puede no terminar nunca). Es un data race: úsalo solo con timeout
// mental y con -race.
var (
	dato      string
	listoMal  bool
	listoBien atomic.Bool
)

func publicarMal() string {
	go func() {
		dato = "hola"
		listoMal = true
	}()
	for !listoMal { // data race
	}
	return dato
}

// publicarBien: Store atómico happens-before el Load que observa true,
// y todo lo escrito antes del Store es visible tras ese Load.
func publicarBien() string {
	go func() {
		dato = "hola"
		listoBien.Store(true)
	}()
	for !listoBien.Load() {
	}
	return dato
}

func main() {
	modo := flag.String("modo", "race", "race|mutex|atomic|channel|publicar-mal|publicar-bien")
	flag.Parse()
	esperado := goroutines * incrementos

	switch *modo {
	case "race":
		fmt.Printf("race:    %d (esperado %d)\n", race(), esperado)
	case "mutex":
		fmt.Printf("mutex:   %d (esperado %d)\n", conMutex(), esperado)
	case "atomic":
		fmt.Printf("atomic:  %d (esperado %d)\n", conAtomic(), esperado)
	case "channel":
		fmt.Printf("channel: %d (esperado %d)\n", conChannel(), esperado)
	case "publicar-mal":
		fmt.Println("publicado:", publicarMal())
	case "publicar-bien":
		fmt.Println("publicado:", publicarBien())
	default:
		flag.Usage()
	}
}
