// Ejercicio 4: iteradores concurrentes (Prefetch y ParMap).
package main

import (
	"fmt"
	"iter"
	"runtime"
	"slices"
	"sync/atomic"
	"time"
)

// fuente produce n elementos tardando d por cada uno.
func fuente(n int, d time.Duration, producidos *atomic.Int32) iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := range n {
			time.Sleep(d)
			producidos.Add(1)
			if !yield(i) {
				return
			}
		}
	}
}

func main() {
	var prod atomic.Int32

	// 1) Prefetch: productor (10 ms/elem) y consumidor (10 ms/elem) se solapan.
	const n = 30
	consumir := func(seq iter.Seq[int]) time.Duration {
		ini := time.Now()
		for range seq {
			time.Sleep(10 * time.Millisecond)
		}
		return time.Since(ini)
	}
	serie := consumir(fuente(n, 10*time.Millisecond, &prod))
	solapado := consumir(Prefetch(fuente(n, 10*time.Millisecond, &prod), 4))
	fmt.Printf("== Prefetch: %d elementos, 10ms producir + 10ms consumir ==\n", n)
	fmt.Printf("  en serie:  %v (≈ %dms)\n", serie.Round(10*time.Millisecond), n*20)
	fmt.Printf("  prefetch:  %v (≈ %dms: se solapan)\n", solapado.Round(10*time.Millisecond), n*10)

	// 2) ParMap: f cuesta 20 ms; 40 elementos.
	entradas := slices.Values(make([]int, 40))
	f := func(int) int { time.Sleep(20 * time.Millisecond); return 1 }
	ini := time.Now()
	for range ParMap(entradas, 1, f) {
	}
	d1 := time.Since(ini)
	ini = time.Now()
	for range ParMap(entradas, 8, f) {
	}
	d8 := time.Since(ini)
	fmt.Printf("\n== ParMap: 40 × 20ms ==\n  1 worker:  %v\n  8 workers: %v (%.1fx)\n", d1.Round(10*time.Millisecond), d8.Round(10*time.Millisecond), float64(d1)/float64(d8))

	// 3) El orden se conserva aunque los cálculos terminen desordenados.
	var orden []int
	for r := range ParMap(slices.Values([]int{5, 1, 4, 2, 3}), 5, func(n int) int {
		time.Sleep(time.Duration(6-n) * 10 * time.Millisecond) // los primeros tardan más
		return n * 10
	}) {
		orden = append(orden, r)
	}
	fmt.Println("\norden de salida de ParMap:", orden)

	// 4) Laziness + cancelación: tomar 3 de una fuente enorme.
	base := runtime.NumGoroutine()
	prod.Store(0)
	for v := range ParMap(fuente(1_000_000, time.Millisecond, &prod), 4, func(n int) int { return n * n }) {
		if v >= 4 { // tras 3 elementos (0, 1, 4)
			break
		}
	}
	fmt.Printf("\nbreak tras 3 resultados: la fuente produjo solo %d de 1000000 elementos\n", prod.Load())
	fmt.Println("goroutines fugadas:", runtime.NumGoroutine()-base)

	// 5) Un panic dentro de f llega a quien itera (y se puede recuperar).
	func() {
		defer func() { fmt.Println("\npanic de f recuperado por el consumidor:", recover()) }()
		for range ParMap(slices.Values([]int{1, 2, 3}), 2, func(n int) int {
			if n == 2 {
				panic("f falló con 2")
			}
			return n
		}) {
		}
	}()
}
