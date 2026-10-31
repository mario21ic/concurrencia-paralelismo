// Ejercicio 5: Ley de Amdahl.
//
//	speedup(N) = 1 / ((1-p) + p/N)
//
// donde p es la fracción paralelizable y N el número de núcleos.
// Primero imprime la tabla teórica y luego mide un programa real con una
// parte serial (sumar resultados con un lock) y otra paralela.
package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

func amdahl(p float64, n int) float64 { return 1 / ((1 - p) + p/float64(n)) }

func trabajoParalelo() int {
	x := 0
	for i := 0; i < 20_000_000; i++ {
		x += i & 7
	}
	return x
}

// ejecutar: `tareas` unidades de trabajo paralelo y, tras cada una, una
// sección serial de duración fija protegida por un mutex.
func ejecutar(workers, tareas int, serial time.Duration) time.Duration {
	var mu sync.Mutex
	var total int
	trabajos := make(chan struct{})
	var wg sync.WaitGroup

	ini := time.Now()
	for range workers {
		wg.Go(func() {
			for range trabajos {
				r := trabajoParalelo()
				mu.Lock()
				total += r
				busy(serial) // sección serial: nadie más entra
				mu.Unlock()
			}
		})
	}
	for range tareas {
		trabajos <- struct{}{}
	}
	close(trabajos)
	wg.Wait()
	return time.Since(ini)
}

// busy espera activamente (sleep tendría granularidad poco fiable).
func busy(d time.Duration) {
	fin := time.Now().Add(d)
	for time.Now().Before(fin) {
	}
}

func main() {
	fmt.Println("Speedup teórico máximo (Amdahl)")
	fmt.Printf("%-10s", "p \\ N")
	ns := []int{1, 2, 4, 8, 16, 64, 1 << 20}
	for _, n := range ns {
		fmt.Printf("%9d", n)
	}
	fmt.Println()
	for _, p := range []float64{0.50, 0.75, 0.90, 0.95, 0.99} {
		fmt.Printf("%-10.2f", p)
		for _, n := range ns {
			fmt.Printf("%9.2f", amdahl(p, n))
		}
		fmt.Println()
	}

	fmt.Println("\nMedición real (con sección serial por tarea)")
	const tareas = 64
	t := time.Now()
	trabajoParalelo()
	unidad := time.Since(t)
	serial := unidad / 5 // ~1/6 serial => p ≈ 0.83
	p := float64(unidad) / float64(unidad+serial)
	fmt.Printf("tarea ≈ %v, parte serial ≈ %v, p ≈ %.2f\n\n", unidad.Round(time.Millisecond), serial.Round(time.Millisecond), p)

	base := ejecutar(1, tareas, serial)
	fmt.Printf("%8s %12s %10s %10s\n", "workers", "tiempo", "speedup", "Amdahl")
	for _, w := range []int{1, 2, 4, runtime.NumCPU()} {
		d := ejecutar(w, tareas, serial)
		fmt.Printf("%8d %12v %9.2fx %9.2fx\n", w, d.Round(time.Millisecond), float64(base)/float64(d), amdahl(p, w))
	}
}
