// Ejercicio 5: cola acotada y ring buffers — channel frente a cola con cerrojo, ring SPSC y ring MPMC.
//
//	go run .
package main

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"
)

const elementos = 4_000_000

// cola abstrae una implementación: Put bloquea (o gira) hasta insertar; Get devuelve false al terminar.
type cola struct {
	put    func(int)
	get    func() (int, bool) // false ⇒ fin de datos
	cerrar func()
}

func medir(nProd, nCons int, c cola) float64 {
	var wp, wc sync.WaitGroup
	t0 := time.Now()
	for range nCons {
		wc.Go(func() {
			for {
				if _, ok := c.get(); !ok {
					return
				}
			}
		})
	}
	for range nProd {
		wp.Go(func() {
			for range elementos / nProd {
				c.put(1)
			}
		})
	}
	wp.Wait()
	c.cerrar()
	wc.Wait()
	return float64(time.Since(t0).Nanoseconds()) / elementos
}

func conChannel(cap int) cola {
	ch := make(chan int, cap)
	return cola{put: func(v int) { ch <- v }, get: func() (int, bool) { v, ok := <-ch; return v, ok }, cerrar: func() { close(ch) }}
}

func conCola(cap int) cola {
	q, ctx := NuevaCola[int](cap), context.Background()
	return cola{
		put:    func(v int) { q.Put(ctx, v) },
		get:    func() (int, bool) { v, err := q.Get(ctx); return v, err == nil },
		cerrar: q.Close,
	}
}

// Los rings no bloquean: el que no puede avanzar GIRA cediendo el procesador. Eso gasta CPU
// cuando el otro extremo es más lento, a cambio de no dormir ni despertar goroutines.
func conRingSPSC(cap int) cola {
	r := NuevoRingSPSC[int](cap)
	var fin chan struct{} = make(chan struct{})
	return cola{
		put: func(v int) {
			for !r.TryPush(v) {
				runtime.Gosched()
			}
		},
		get: func() (int, bool) {
			for {
				if v, ok := r.TryPop(); ok {
					return v, true
				}
				select {
				case <-fin:
					v, ok := r.TryPop()
					return v, ok
				default:
					runtime.Gosched()
				}
			}
		},
		cerrar: func() { close(fin) },
	}
}

func conRingMPMC(cap int) cola {
	r := NuevoRingMPMC[int](cap)
	fin := make(chan struct{})
	return cola{
		put: func(v int) {
			for !r.TryPush(v) {
				runtime.Gosched()
			}
		},
		get: func() (int, bool) {
			for {
				if v, ok := r.TryPop(); ok {
					return v, true
				}
				select {
				case <-fin:
					return r.TryPop()
				default:
					runtime.Gosched()
				}
			}
		},
		cerrar: func() { close(fin) },
	}
}

func mejor(veces int, f func() float64) float64 {
	m := 1e18
	for range veces {
		m = min(m, f())
	}
	return m
}

func main() {
	fmt.Printf("%d elementos, buffer de 1024, GOMAXPROCS=%d (mejor de 3 ejecuciones, ns por elemento)\n\n", elementos, runtime.GOMAXPROCS(0))

	fmt.Println("1 productor + 1 consumidor:")
	spsc := []struct {
		n string
		f func(int) cola
	}{{"chan int", conChannel}, {"Cola (mutex+cond)", conCola}, {"RingSPSC (sin cerrojos)", conRingSPSC}, {"RingMPMC (Vyukov)", conRingMPMC}}
	for _, c := range spsc {
		fmt.Printf("  %-26s %7.1f ns\n", c.n, mejor(3, func() float64 { return medir(1, 1, c.f(1024)) }))
	}

	fmt.Println("\n4 productores + 4 consumidores:")
	for _, c := range []struct {
		n string
		f func(int) cola
	}{{"chan int", conChannel}, {"Cola (mutex+cond)", conCola}, {"RingMPMC (Vyukov)", conRingMPMC}} {
		fmt.Printf("  %-26s %7.1f ns\n", c.n, mejor(3, func() float64 { return medir(4, 4, c.f(1024)) }))
	}
}
