// Ejercicio 2: CPU-bound vs I/O-bound.
//
// Varía el número de workers y observa dónde deja de mejorar el tiempo
// según el tipo de carga.
package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

const totalTareas = 64

func cpuTask() {
	x := 0
	for i := 0; i < 50_000_000; i++ {
		x += i
	}
	_ = x
}

func ioTask() { time.Sleep(50 * time.Millisecond) }

// correr reparte totalTareas entre `workers` goroutines.
func correr(workers int, tarea func()) time.Duration {
	ini := time.Now()
	trabajos := make(chan struct{})
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for range trabajos {
				tarea()
			}
		})
	}
	for range totalTareas {
		trabajos <- struct{}{}
	}
	close(trabajos)
	wg.Wait()
	return time.Since(ini)
}

func main() {
	n := runtime.NumCPU()
	fmt.Printf("NumCPU=%d, tareas=%d\n\n", n, totalTareas)
	fmt.Printf("%8s %14s %14s\n", "workers", "CPU-bound", "I/O-bound")
	for _, w := range []int{1, 2, 4, n, 2 * n, 8 * n, 64} {
		fmt.Printf("%8d %14v %14v\n", w,
			correr(w, cpuTask).Round(time.Millisecond),
			correr(w, ioTask).Round(time.Millisecond))
	}
}
