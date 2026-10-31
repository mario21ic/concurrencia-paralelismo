// Ejercicio 3: sync.WaitGroup, WaitGroup.Go y errores típicos.
//
//	go run .
//	go run -race . -modo=add-dentro   # Add dentro de la goroutine
package main

import (
	"flag"
	"fmt"
	"sync"
	"sync/atomic"
)

// clasico: forma anterior a Go 1.25.
func clasico(n int) int64 {
	var wg sync.WaitGroup
	var hechas atomic.Int64
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			hechas.Add(1)
		}()
	}
	wg.Wait()
	return hechas.Load()
}

// moderno: Go 1.25+, Add(1) + go + defer Done en una sola llamada.
func moderno(n int) int64 {
	var wg sync.WaitGroup
	var hechas atomic.Int64
	for range n {
		wg.Go(func() { hechas.Add(1) })
	}
	wg.Wait()
	return hechas.Load()
}

// addDentro: BUG. Wait puede ejecutarse antes de que cualquier Add ocurra.
func addDentro(n int) int64 {
	var wg sync.WaitGroup
	var hechas atomic.Int64
	for range n {
		go func() {
			wg.Add(1) // demasiado tarde
			defer wg.Done()
			hechas.Add(1)
		}()
	}
	wg.Wait()
	return hechas.Load()
}

// seguro: un panic en una goroutine sin recover tumba TODO el proceso.
// Este envoltorio lo convierte en un error.
func seguro(wg *sync.WaitGroup, errs chan<- error, f func()) {
	wg.Go(func() {
		defer func() {
			if r := recover(); r != nil {
				errs <- fmt.Errorf("panic recuperado: %v", r)
			}
		}()
		f()
	})
}

func main() {
	modo := flag.String("modo", "todo", "todo|add-dentro")
	flag.Parse()

	if *modo == "add-dentro" {
		incompletas, panics := 0, 0
		for range 200 {
			func() {
				defer func() {
					if recover() != nil {
						panics++ // "WaitGroup is reused before previous Wait has returned"
					}
				}()
				if addDentro(100) != 100 {
					incompletas++
				}
			}()
		}
		fmt.Printf("add-dentro, 200 ejecuciones: Wait retornó antes de tiempo %d veces, panic del runtime %d veces\n", incompletas, panics)
		return
	}

	fmt.Println("clásico:", clasico(1000))
	fmt.Println("moderno:", moderno(1000))

	var wg sync.WaitGroup
	errs := make(chan error, 3)
	for i := range 3 {
		seguro(&wg, errs, func() {
			if i == 1 {
				panic("algo salió mal")
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		fmt.Println("error:", err)
	}
}
