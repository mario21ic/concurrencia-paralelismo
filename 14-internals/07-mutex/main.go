// Ejercicio 7: sync.Mutex por dentro — modo normal frente a modo de inanición (starvation).
//
//	go run .
package main

import (
	"fmt"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

type locker interface {
	Lock()
	Unlock()
}

// spin es un cerrojo CAS puro: sin cola, sin modo de inanición, sin dormir.
type spin struct{ v atomic.Uint32 }

func (s *spin) Lock() {
	for !s.v.CompareAndSwap(0, 1) {
	}
}
func (s *spin) Unlock() { s.v.Store(0) }

// ocupar simula trabajo dentro de la sección crítica.
//
//go:noinline
func ocupar(d time.Duration) {
	fin := time.Now().Add(d)
	for time.Now().Before(fin) {
	}
}

type resultado struct {
	porHilo []int64         // adquisiciones de cada "acaparador"
	esperas []time.Duration // esperas de la víctima
}

// escenario: `acaparadores` goroutines toman el cerrojo en bucle sin pausa; una víctima intenta
// tomarlo cada 5 ms y mide cuánto espera.
func escenario(l locker, acaparadores int, seccion time.Duration, dur time.Duration) resultado {
	var parar atomic.Bool
	var wg sync.WaitGroup
	cuentas := make([]int64, acaparadores)
	for i := range acaparadores {
		wg.Go(func() {
			for !parar.Load() {
				l.Lock()
				ocupar(seccion)
				l.Unlock()
				cuentas[i]++
			}
		})
	}
	var esperas []time.Duration
	fin := time.Now().Add(dur)
	for time.Now().Before(fin) {
		time.Sleep(5 * time.Millisecond)
		t0 := time.Now()
		l.Lock()
		esperas = append(esperas, time.Since(t0))
		l.Unlock()
	}
	parar.Store(true)
	wg.Wait()
	return resultado{cuentas, esperas}
}

func pct(d []time.Duration, p float64) time.Duration {
	s := slices.Clone(d)
	slices.Sort(s)
	return s[int(float64(len(s)-1)*p)]
}

func main() {
	fmt.Printf("sizeof(sync.Mutex) = %d bytes (estado int32 + semáforo uint32)\n", unsafe.Sizeof(sync.Mutex{}))
	fmt.Println(`  bits del estado: [ nº de esperando… | starving | woken | locked ]`)
	fmt.Println("  modo normal: el que llega puede 'colarse' (barging) antes que el despertado → más rendimiento, injusto")
	fmt.Println("  modo inanición: si un esperando lleva >1 ms, el cerrojo se entrega EN ORDEN (FIFO) y nadie se cuela")
	fmt.Println()

	acap := max(2, runtime.GOMAXPROCS(0)-2)
	const dur = 2 * time.Second
	for _, seccion := range []time.Duration{200 * time.Nanosecond, 50 * time.Microsecond} {
		fmt.Printf("== %d goroutines acaparando, sección crítica de %v, víctima cada 5 ms (%v) ==\n", acap, seccion, dur)
		for _, c := range []struct {
			nombre string
			l      locker
		}{
			{"sync.Mutex", new(sync.Mutex)},
			{"spinlock CAS", new(spin)},
		} {
			r := escenario(c.l, acap, seccion, dur)
			var total int64
			mn, mx := r.porHilo[0], r.porHilo[0]
			for _, v := range r.porHilo {
				total += v
				mn, mx = min(mn, v), max(mx, v)
			}
			fmt.Printf("  %-13s víctima: p50=%-9v p99=%-9v máx=%-9v | acaparadores: %d ops (reparto mín/máx = %.2f)\n",
				c.nombre, pct(r.esperas, .5), pct(r.esperas, .99), slices.Max(r.esperas), total, float64(mn)/float64(max(mx, 1)))
		}
		fmt.Println()
	}
	fmt.Println("Lectura: el modo inanición pone una COTA a la espera (≈ 1 ms + el tiempo de las secciones previas).")
	fmt.Println("Un cerrojo sin cola no ofrece garantía: la espera de la víctima depende de la suerte con el CAS.")
	fmt.Println("Contrapartida: en modo inanición cada traspaso despierta a una goroutine (más lento por operación): por eso el modo normal es el predeterminado.")
}
