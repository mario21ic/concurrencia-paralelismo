// Caso 3.3 (avanzado): repartir el trabajo entre N workers (fan-out) y recuperar los resultados
// EN EL ORDEN DE ENTRADA (fan-in ordenado), con memoria acotada.
//
// Problema: transformar un flujo de imágenes con 4 workers. Los workers terminan en desorden (unas
// tardan más), pero el cliente necesita los resultados en el orden original (p. ej. fotogramas de un
// vídeo). Un simple fan-in los entregaría mezclados.
//
// Solución: numerar la entrada, procesar en paralelo y reordenar en el recolector. Una «ventana»
// limita cuántos elementos pueden estar en vuelo, para que un elemento lento no haga crecer sin
// límite el búfer de reordenación (contrapresión).
//
//	go run .
package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type numerado struct {
	seq int
	val int
}

// Procesar aplica f a cada elemento de `in` con `workers` goroutines y entrega los resultados en el
// mismo orden. Como mucho `2*workers` elementos están en vuelo a la vez.
func Procesar(ctx context.Context, in <-chan int, workers int, f func(int) int) <-chan int {
	ventana := make(chan struct{}, 2*workers) // plazas en vuelo
	trabajo := make(chan numerado)
	hecho := make(chan numerado)
	out := make(chan int)

	// Distribuidor: numera y reparte (solo avanza si queda plaza en la ventana).
	go func() {
		defer close(trabajo)
		seq := 0
		for v := range in {
			select {
			case ventana <- struct{}{}:
			case <-ctx.Done():
				return
			}
			select {
			case trabajo <- numerado{seq, v}:
				seq++
			case <-ctx.Done():
				return
			}
		}
	}()

	// Workers: fan-out.
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for t := range trabajo {
				r := numerado{t.seq, f(t.val)}
				select {
				case hecho <- r:
				case <-ctx.Done():
					return
				}
			}
		})
	}
	go func() { wg.Wait(); close(hecho) }()

	// Recolector: fan-in con reordenación.
	go func() {
		defer close(out)
		pendientes := map[int]int{}
		siguiente := 0
		for r := range hecho {
			pendientes[r.seq] = r.val
			for {
				v, ok := pendientes[siguiente]
				if !ok {
					break
				}
				delete(pendientes, siguiente)
				siguiente++
				select {
				case out <- v:
					<-ventana // libera una plaza: el distribuidor puede admitir otro elemento
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out
}

func main() {
	entrada := make(chan int)
	go func() {
		defer close(entrada)
		for i := range 12 {
			entrada <- i
		}
	}()
	// El elemento 0 es 10 veces más lento que el resto.
	lento := func(v int) int {
		if v == 0 {
			time.Sleep(100 * time.Millisecond)
		} else {
			time.Sleep(10 * time.Millisecond)
		}
		return v * v
	}
	t0 := time.Now()
	var salida []int
	for v := range Procesar(context.Background(), entrada, 4, lento) {
		salida = append(salida, v)
	}
	fmt.Printf("resultados (en orden de entrada): %v\n", salida)
	fmt.Printf("tardó %v (secuencial: %v)\n", time.Since(t0).Round(10*time.Millisecond), 100*time.Millisecond+11*10*time.Millisecond)
}
