// Ejercicio 4: productor–consumidor, contrapresión y batching.
package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type resultado struct {
	productoresTerminaron time.Duration
	total                 time.Duration
	maxCola               int
	bloqueado             time.Duration
}

// simular: 3 productores × 20 elementos (rápidos) y 2 consumidores (10 ms/elemento).
func simular(bufer int) resultado {
	cola := make(chan int, bufer)
	var bloqueado atomic.Int64 // ns que los productores pasaron esperando
	var maxCola atomic.Int64

	ini := time.Now()
	stop := make(chan struct{})
	go func() { // muestreador de la longitud de la cola
		for {
			select {
			case <-stop:
				return
			default:
				if l := int64(len(cola)); l > maxCola.Load() {
					maxCola.Store(l)
				}
				time.Sleep(200 * time.Microsecond)
			}
		}
	}()

	var wgP, wgC sync.WaitGroup
	for range 3 {
		wgP.Go(func() {
			for i := range 20 {
				t := time.Now()
				cola <- i // se bloquea si la cola está llena => CONTRAPRESIÓN
				bloqueado.Add(int64(time.Since(t)))
			}
		})
	}
	for range 2 {
		wgC.Go(func() {
			for range cola {
				time.Sleep(10 * time.Millisecond)
			}
		})
	}
	wgP.Wait()
	prod := time.Since(ini)
	close(cola) // los productores terminaron: el coordinador cierra
	wgC.Wait()
	close(stop)
	return resultado{prod, time.Since(ini), int(maxCola.Load()), time.Duration(bloqueado.Load())}
}

func main() {
	fmt.Println("== 3 productores rápidos, 2 consumidores de 10ms/elemento (60 elementos) ==")
	fmt.Printf("%8s %22s %12s %12s %20s\n", "buffer", "productores acabaron", "total", "cola máx.", "productores bloqueados")
	for _, b := range []int{0, 5, 100} {
		r := simular(b)
		fmt.Printf("%8d %22v %12v %12d %20v\n", b, r.productoresTerminaron.Round(time.Millisecond), r.total.Round(time.Millisecond), r.maxCola, r.bloqueado.Round(time.Millisecond))
	}

	fmt.Println("\n== batching: lotes de hasta 4, o a los 50ms ==")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	in := make(chan int)
	go func() {
		defer close(in)
		for i := 1; i <= 10; i++ { // ráfaga de 10 rápidos
			in <- i
		}
		time.Sleep(120 * time.Millisecond)
		in <- 11 // elemento aislado: se emitirá por tiempo
		in <- 12
	}()
	ini := time.Now()
	for lote := range Batch(ctx, in, 4, 50*time.Millisecond) {
		fmt.Printf("  [%v] lote: %v\n", time.Since(ini).Round(10*time.Millisecond), lote)
	}
}
