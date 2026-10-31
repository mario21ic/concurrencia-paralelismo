// Ejercicio 8: lock-free — ABA, bucles CAS y los límites de la técnica.
//
//	go run .
package main

import (
	"fmt"
	"runtime"
	"slices"
	"sync"
	"time"
)

func main() {
	fmt.Println("== A) el problema ABA (entrelazado determinista sobre una pila con huecos reutilizables) ==")
	for _, etq := range []bool{false, true} {
		cas, antes, despues := EscenarioABA(etq)
		fmt.Printf("  etiqueta de versión=%-5v pila inicial %s | T1 hace CAS(A→B) tras la interferencia de T2: %-9s → pila: %s\n",
			etq, antes, map[bool]string{true: "TRIUNFA", false: "FALLA"}[cas], despues)
	}
	fmt.Println("  Sin etiqueta, la pila queda en [B → C]: B ya había sido extraído (se 'resucita') y A' se pierde.")
	fmt.Println("  En Go con punteros y GC el problema no ocurre (la memoria no se reutiliza mientras alguien la apunte);")
	fmt.Println("  reaparece con índices en arrays, pools propios, memoria compartida o unsafe.")

	g := runtime.GOMAXPROCS(0)
	fmt.Printf("\n== B) incrementar un contador: mutex, bucle CAS y Add (%d M de incrementos en total) ==\n", 8)
	fmt.Printf("  %-3s %-24s %-24s %-24s %s\n", "G", "mutex", "bucle CAS", "atomic.Add", "reintentos CAS por op (media / máx)")
	for _, gs := range []int{1, 2, 4, 8} {
		const total = 8_000_000
		tm := ejecutar(gs, total, func() func() { var c ContadorMutex; return c.Inc })
		var c ContadorCAS
		var mu sync.Mutex
		var reint, maxReint int
		tc := ejecutarN(gs, total, func(n int) {
			var r, mx int
			for range n {
				x := c.Inc()
				r += x
				mx = max(mx, x)
			}
			mu.Lock()
			reint += r
			maxReint = max(maxReint, mx)
			mu.Unlock()
		})
		ta := ejecutar(gs, total, func() func() { var c ContadorAdd; return c.Inc })
		fmt.Printf("  %-3d %-24s %-24s %-24s %.2f / %d\n", gs, nsop(tm, total), nsop(tc, total), nsop(ta, total), float64(reint)/total, maxReint)
	}
	fmt.Println("  Lock-free ≠ más rápido ni justo: con contención el CAS reintenta (trabajo desperdiciado) y una goroutine")
	fmt.Println("  concreta puede reintentar muchas veces seguidas (no es wait-free). atomic.Add no reintenta nunca.")

	fmt.Printf("\n== C) pila y cola: lock-free frente a mutex y channel (%d productores + %d consumidores, 1 M elementos cada lado) ==\n", g/2, g/2)
	const por = 1_000_000
	p := g / 2
	fmt.Printf("  %-26s %s\n", "pila Treiber (lock-free)", nsop(parejas(p, por, func() (func(int), func() bool) {
		var s PilaTreiber[int]
		return s.Push, func() bool { _, ok := s.Pop(); return ok }
	}), p*por))
	fmt.Printf("  %-26s %s\n", "pila con mutex", nsop(parejas(p, por, func() (func(int), func() bool) {
		var s PilaMutex[int]
		return s.Push, func() bool { _, ok := s.Pop(); return ok }
	}), p*por))
	fmt.Printf("  %-26s %s\n", "cola Michael-Scott", nsop(parejas(p, por, func() (func(int), func() bool) {
		q := NuevaColaMS[int]()
		return q.Enqueue, func() bool { _, ok := q.Dequeue(); return ok }
	}), p*por))
	fmt.Printf("  %-26s %s\n", "cola con mutex", nsop(parejas(p, por, func() (func(int), func() bool) {
		var q ColaMutex[int]
		return q.Enqueue, func() bool { _, ok := q.Dequeue(); return ok }
	}), p*por))
	fmt.Printf("  %-26s %s\n", "chan int (buffer 1024)", nsop(parejasChan(p, por), p*por))
	fmt.Println("  (cada elemento = una inserción + una extracción; las estructuras lock-free asignan un nodo por elemento)")
}

func nsop(d time.Duration, n int) string {
	return fmt.Sprintf("%.1f ns/op", float64(d.Nanoseconds())/float64(n))
}

func ejecutar(g, total int, nuevo func() func()) time.Duration {
	inc := nuevo()
	return ejecutarN(g, total, func(n int) {
		for range n {
			inc()
		}
	})
}

func ejecutarN(g, total int, f func(n int)) time.Duration {
	var wg sync.WaitGroup
	t0 := time.Now()
	for range g {
		wg.Go(func() { f(total / g) })
	}
	wg.Wait()
	return time.Since(t0)
}

// parejas: p productores insertan `por` elementos cada uno y p consumidores sacan `por` cada uno.
func parejas(p, por int, nuevo func() (func(int), func() bool)) time.Duration {
	var mejores []time.Duration
	for range 3 {
		push, pop := nuevo()
		var wg sync.WaitGroup
		t0 := time.Now()
		for range p {
			wg.Go(func() {
				for i := range por {
					push(i)
				}
			})
			wg.Go(func() {
				for n := 0; n < por; {
					if pop() {
						n++
					} else {
						runtime.Gosched()
					}
				}
			})
		}
		wg.Wait()
		mejores = append(mejores, time.Since(t0))
	}
	return slices.Min(mejores)
}

func parejasChan(p, por int) time.Duration {
	var mejores []time.Duration
	for range 3 {
		ch := make(chan int, 1024)
		var wg sync.WaitGroup
		t0 := time.Now()
		for range p {
			wg.Go(func() {
				for i := range por {
					ch <- i
				}
			})
			wg.Go(func() {
				for range por {
					<-ch
				}
			})
		}
		wg.Wait()
		mejores = append(mejores, time.Since(t0))
	}
	return slices.Min(mejores)
}
