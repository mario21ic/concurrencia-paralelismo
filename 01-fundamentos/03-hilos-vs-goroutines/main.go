// Ejercicio 3: costo de una goroutine.
//
// Lanza N goroutines bloqueadas y mide memoria y tiempo de creación.
// Un hilo de SO típico reserva ~1-8 MB de stack; una goroutine empieza
// con ~2 KB y crece dinámicamente.
package main

import (
	"flag"
	"fmt"
	"runtime"
	"sync"
	"time"
)

func main() {
	n := flag.Int("n", 100_000, "número de goroutines")
	flag.Parse()

	var antes, despues runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&antes)

	parar := make(chan struct{})
	var listas, wg sync.WaitGroup
	listas.Add(*n)

	ini := time.Now()
	for range *n {
		wg.Go(func() {
			listas.Done()
			<-parar // bloqueada: mantiene su stack vivo
		})
	}
	listas.Wait()
	creacion := time.Since(ini)

	runtime.ReadMemStats(&despues)
	stack := despues.StackInuse - antes.StackInuse

	fmt.Printf("goroutines vivas:      %d\n", runtime.NumGoroutine()-1)
	fmt.Printf("tiempo de creación:    %v (%v por goroutine)\n", creacion.Round(time.Millisecond), creacion/time.Duration(*n))
	fmt.Printf("stack total:           %.1f MB\n", float64(stack)/1e6)
	fmt.Printf("stack por goroutine:   ~%.0f bytes\n", float64(stack)/float64(*n))
	fmt.Printf("referencia: %d hilos de SO de 8 MB reservarían %.1f GB (virtual)\n", *n, float64(*n)*8/1024)

	close(parar)
	wg.Wait()
}
