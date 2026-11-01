// Caso 2.3 (avanzado): un contador de métricas que recibe millones de incrementos por segundo.
//
// Problema: 8 goroutines incrementan el MISMO contador sin parar. Un mutex funciona; un atomic es más
// rápido; pero ambos siguen peleando por UNA línea de caché. Lo más rápido es no compartir: cada
// goroutine suma en su propio fragmento y se suman al leer.
//
// Idea del módulo 13: «la mejor sincronización es la que no existe».
//
//	go run .
package main

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sys/cpu"
)

type Contador interface {
	Inc(id int) // id: goroutine que incrementa (solo lo usa el fragmentado)
	Total() int64
}

type conMutex struct {
	mu sync.Mutex
	n  int64
}

func (c *conMutex) Inc(int) { c.mu.Lock(); c.n++; c.mu.Unlock() }
func (c *conMutex) Total() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

type conAtomic struct{ n atomic.Int64 }

func (c *conAtomic) Inc(int)      { c.n.Add(1) }
func (c *conAtomic) Total() int64 { return c.n.Load() }

// fragmentado: un contador por goroutine, cada uno en su PROPIA línea de caché (relleno), para que
// dos núcleos no se disputen la misma línea ("false sharing").
type fragmentado struct {
	frag []struct {
		n atomic.Int64
		_ cpu.CacheLinePad
	}
}

func nuevoFragmentado(n int) *fragmentado {
	return &fragmentado{frag: make([]struct {
		n atomic.Int64
		_ cpu.CacheLinePad
	}, n)}
}
func (c *fragmentado) Inc(id int) { c.frag[id].n.Add(1) }
func (c *fragmentado) Total() (t int64) {
	for i := range c.frag {
		t += c.frag[i].n.Load()
	}
	return
}

// Medir lanza g goroutines que incrementan `total/g` veces cada una y devuelve ns por incremento.
func Medir(c Contador, g, total int) float64 {
	var wg sync.WaitGroup
	t0 := time.Now()
	for id := range g {
		wg.Go(func() {
			for range total / g {
				c.Inc(id)
			}
		})
	}
	wg.Wait()
	return float64(time.Since(t0).Nanoseconds()) / float64(total)
}

func main() {
	const total = 16_000_000
	g := runtime.GOMAXPROCS(0)
	fmt.Printf("%d goroutines, %d incrementos (ns por incremento; menos es mejor)\n", g, total)
	for _, c := range []struct {
		nombre string
		c      Contador
	}{
		{"mutex", &conMutex{}},
		{"atomic.Int64 compartido", &conAtomic{}},
		{"fragmentado (uno por goroutine)", nuevoFragmentado(g)},
	} {
		ns := min(Medir(c.c, g, total), Medir(c.c, g, total), Medir(c.c, g, total))
		fmt.Printf("  %-34s %7.2f ns\n", c.nombre, ns)
	}
}
