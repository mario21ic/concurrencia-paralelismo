// Ejercicio 2: sync.RWMutex — cuándo ayuda y cuándo NO.
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

type almacen interface {
	Leer(k int) int
	Escribir(k, v int)
}

// trabajo simula cuánto tarda la sección crítica.
func trabajo(n int) {
	x := 0
	for i := 0; i < n; i++ {
		x += i
	}
	sink = x
}

var sink int

type conMutex struct {
	mu   sync.Mutex
	m    map[int]int
	work int
}

func (s *conMutex) Leer(k int) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	trabajo(s.work)
	return s.m[k]
}
func (s *conMutex) Escribir(k, v int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	trabajo(s.work)
	s.m[k] = v
}

type conRWMutex struct {
	mu   sync.RWMutex
	m    map[int]int
	work int
}

func (s *conRWMutex) Leer(k int) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	trabajo(s.work)
	return s.m[k]
}
func (s *conRWMutex) Escribir(k, v int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	trabajo(s.work)
	s.m[k] = v
}

func correr(s almacen, goroutines, ops int, pctLecturas int) time.Duration {
	var wg sync.WaitGroup
	ini := time.Now()
	for g := range goroutines {
		wg.Go(func() {
			r := rand.New(rand.NewPCG(uint64(g), 1))
			for range ops {
				k := r.IntN(100)
				if r.IntN(100) < pctLecturas {
					s.Leer(k)
				} else {
					s.Escribir(k, k)
				}
			}
		})
	}
	wg.Wait()
	return time.Since(ini)
}

// mejorDe repite la medición y se queda con la menor (reduce el ruido del planificador),
// alternando el orden Mutex/RWMutex para que ninguno parta siempre "en frío".
func tabla(titulo string, work int) {
	g := runtime.NumCPU()
	const ops, reps = 200_000, 5
	fmt.Printf("\n%s (sección crítica ≈ %d iteraciones, %d goroutines, mejor de %d)\n", titulo, work, g, reps)
	fmt.Printf("%12s %12s %12s %10s\n", "% lecturas", "Mutex", "RWMutex", "RW/Mutex")
	for _, pct := range []int{50, 90, 99, 100} {
		dm, drw := time.Duration(1<<62), time.Duration(1<<62)
		for r := range reps {
			m := &conMutex{m: map[int]int{}, work: work}
			rw := &conRWMutex{m: map[int]int{}, work: work}
			if r%2 == 0 {
				dm = min(dm, correr(m, g, ops, pct))
				drw = min(drw, correr(rw, g, ops, pct))
			} else {
				drw = min(drw, correr(rw, g, ops, pct))
				dm = min(dm, correr(m, g, ops, pct))
			}
		}
		fmt.Printf("%11d%% %12v %12v %9.2fx\n", pct, dm.Round(time.Millisecond), drw.Round(time.Millisecond), float64(drw)/float64(dm))
	}
}

// reentrada: un RLock "recursivo" puede producir deadlock si hay un escritor esperando.
func reentrada() {
	var mu sync.RWMutex
	listo := make(chan struct{})
	hecho := make(chan struct{})

	go func() {
		mu.RLock()
		close(listo)
		time.Sleep(50 * time.Millisecond) // da tiempo a que llegue el escritor
		mu.RLock()                        // 2.º RLock: queda detrás del escritor pendiente
		mu.RUnlock()
		mu.RUnlock()
		close(hecho)
	}()

	<-listo
	go func() {
		mu.Lock() // escritor: espera a que se libere el 1.er RLock y bloquea nuevos lectores
		mu.Unlock()
	}()

	select {
	case <-hecho:
		fmt.Println("sin deadlock")
	case <-time.After(300 * time.Millisecond):
		fmt.Println("DEADLOCK: el lector espera al escritor y el escritor espera al lector (RLock recursivo)")
	}
}

func main() {
	tabla("Sección crítica CORTA (lookup de map)", 0)
	tabla("Sección crítica LARGA (lectura costosa)", 2000)

	fmt.Println("\n== RLock recursivo ==")
	reentrada()
}
