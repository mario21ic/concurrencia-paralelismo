// Ejercicio 7: skip list concurrente (perezosa) frente a un slice ordenado con RWMutex.
//
//	go run .
package main

import (
	"fmt"
	"math/rand/v2"
	"sync"
	"time"
)

const (
	rango = 40_000
	ops   = 2_000_000
)

type nuevoConjunto func() Conjunto

func medir(nuevo nuevoConjunto, g, pctContains int) float64 {
	c := nuevo()
	r := rand.New(rand.NewPCG(1, 1))
	for range rango / 2 { // mitad llena
		c.Add(r.IntN(rango))
	}
	var wg sync.WaitGroup
	t0 := time.Now()
	for id := range g {
		wg.Go(func() {
			r := rand.New(rand.NewPCG(uint64(id), 5))
			for range ops / g {
				k := r.IntN(rango)
				switch p := r.IntN(100); {
				case p < pctContains:
					c.Contains(k)
				case p < pctContains+(100-pctContains)/2:
					c.Add(k)
				default:
					c.Remove(k)
				}
			}
		})
	}
	wg.Wait()
	return float64(time.Since(t0).Nanoseconds()) / ops
}

func mejor(f func() float64) float64 { return min(f(), f(), f()) }

func main() {
	fmt.Printf("conjunto ordenado de enteros, claves en [0,%d), mitad llena, %d operaciones (ns por operación, mejor de 3)\n", rango, ops)
	impl := []struct {
		n string
		f nuevoConjunto
	}{
		{"slice ordenado + RWMutex", func() Conjunto { return new(SliceMutex) }},
		{"skip list perezosa", func() Conjunto { return adaptador{Nueva[int]()} }},
	}
	for _, pct := range []int{90, 50} {
		fmt.Printf("\n%d %% Contains, %d %% Add, %d %% Remove:\n", pct, (100-pct)/2, (100-pct)/2)
		fmt.Printf("  %-28s %8s %8s %8s %8s\n", "goroutines →", "1", "2", "4", "8")
		for _, im := range impl {
			fmt.Printf("  %-28s", im.n)
			for _, g := range []int{1, 2, 4, 8} {
				fmt.Printf(" %8.1f", mejor(func() float64 { return medir(im.f, g, pct) }))
			}
			fmt.Println()
		}
	}
}

type adaptador struct{ *Lista[int] }
