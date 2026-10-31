// Ejercicio 6: hash map con sharding frente a un RWMutex global y sync.Map.
//
//	go run .
package main

import (
	"fmt"
	"math/rand/v2"
	"runtime"
	"sync"
	"time"
)

const (
	claves = 1 << 16
	ops    = 8_000_000
)

// medir: `g` goroutines hacen ops/g operaciones; lecturaPct % son Load y el resto Store.
// hot > 0: ese % de las operaciones va a UNA clave caliente.
func medir(m Mapa, g, lecturaPct, hot int) float64 {
	for k := range claves {
		m.Store(k, k)
	}
	var wg sync.WaitGroup
	t0 := time.Now()
	for id := range g {
		wg.Go(func() {
			r := rand.New(rand.NewPCG(uint64(id), 7))
			for range ops / g {
				k := r.IntN(claves)
				if hot > 0 && r.IntN(100) < hot {
					k = 0
				}
				if r.IntN(100) < lecturaPct {
					m.Load(k)
				} else {
					m.Store(k, k)
				}
			}
		})
	}
	wg.Wait()
	return float64(time.Since(t0).Nanoseconds()) / ops
}

func mejor(f func() float64) float64 { return min(f(), f(), f()) }

func main() {
	g := runtime.GOMAXPROCS(0)
	fmt.Printf("%d goroutines, %d claves, %d operaciones (ns por operación del conjunto, mejor de 3)\n", g, claves, ops)

	for _, w := range []struct {
		nombre    string
		lect, hot int
	}{
		{"95 % lecturas, claves uniformes", 95, 0},
		{"50 % lecturas, claves uniformes", 50, 0},
		{"50 % lecturas, 30 % de las ops en UNA clave caliente", 50, 30},
	} {
		fmt.Printf("\n%s:\n", w.nombre)
		fmt.Printf("  %-26s %7.1f ns\n", "RWMutex global", mejor(func() float64 { return medir(NuevoGlobal(), g, w.lect, w.hot) }))
		fmt.Printf("  %-26s %7.1f ns\n", "sync.Map", mejor(func() float64 { return medir(new(MapaSync), g, w.lect, w.hot) }))
		for _, n := range []int{4, 16, 64, 256} {
			fmt.Printf("  %-26s %7.1f ns\n", fmt.Sprintf("Fragmentado (%d frag.)", n), mejor(func() float64 { return medir(NuevoFragInt(n), g, w.lect, w.hot) }))
		}
	}
}
