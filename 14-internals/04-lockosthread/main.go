// Ejercicio 4: runtime.LockOSThread — fijar una goroutine a un hilo del SO.
//
//	go run .
package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

var tidInit = tid() // se calcula en la inicialización del paquete (goroutine principal)

// hilosVistos anota en qué hilos del SO se ejecuta una goroutine a lo largo de 200 pausas.
func hilosVistos(fijar bool) map[int]int {
	vistos := map[int]int{}
	var wg sync.WaitGroup
	wg.Go(func() {
		if fijar {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
		}
		for range 200 {
			vistos[tid()]++
			time.Sleep(200 * time.Microsecond) // se desbloquea y puede reanudarse en OTRO hilo
		}
	})
	// ruido para que haya más de un hilo disponible
	for range 4 {
		wg.Go(func() {
			for range 200 {
				time.Sleep(200 * time.Microsecond)
			}
		})
	}
	wg.Wait()
	return vistos
}

func pingPong(vueltas int, fijar bool) time.Duration {
	a, b := make(chan int), make(chan int)
	var wg sync.WaitGroup
	wg.Go(func() {
		if fijar {
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()
		}
		for v := range a {
			b <- v + 1
		}
		close(b)
	})
	if fijar {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
	}
	ini := time.Now()
	for i := range vueltas {
		a <- i
		<-b
	}
	d := time.Since(ini)
	close(a)
	wg.Wait()
	return d
}

// hilosDistintos ejecuta n goroutines SUCESIVAS que se fijan a su hilo y anotan en cuál están.
// Si el hilo se destruye al terminar la goroutine, cada una recibe un hilo nuevo.
func hilosDistintos(n int, soltar bool) int {
	vistos := map[int]bool{}
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			runtime.LockOSThread()
			vistos[tid()] = true
			if soltar {
				runtime.UnlockOSThread()
			}
		})
		wg.Wait()
	}
	return len(vistos)
}

func main() {
	if !tidDisponible {
		fmt.Println("este ejercicio necesita identificar hilos del SO (solo macOS y Linux)")
		return
	}
	fmt.Printf("NumCPU=%d, hilo del SO de la goroutine principal: %d\n", runtime.NumCPU(), tidInit)

	fmt.Println("\n== 1) ¿en cuántos hilos del SO se ejecuta una goroutine que se duerme 200 veces? ==")
	sin := hilosVistos(false)
	con := hilosVistos(true)
	fmt.Printf("  sin LockOSThread: %d hilos distintos\n", len(sin))
	fmt.Printf("  con LockOSThread: %d hilo\n", len(con))

	fmt.Println("\n== 2) el patrón del hilo principal (GUI/OpenGL): ¿dónde se ejecutan las llamadas? ==")
	fmt.Printf("  main() está en el hilo %d (init estaba en el %d: %v)\n", tid(), tidInit, tid() == tidInit)
	parar := make(chan struct{})
	resultados := make(chan int, 10)
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() { // 10 goroutines distintas piden ejecutar algo "en el hilo principal"
			EnHiloPrincipal(func() { resultados <- tid() })
		})
	}
	go func() { wg.Wait(); close(parar) }()
	BucleDelHiloPrincipal(parar) // main atiende las peticiones
	enPrincipal := 0
	for range 10 {
		if <-resultados == tidInit {
			enPrincipal++
		}
	}
	fmt.Printf("  %d de 10 llamadas se ejecutaron en el hilo principal\n", enPrincipal)

	fmt.Println("\n== 3) coste: ping-pong entre dos goroutines (200 000 vueltas) ==")
	runtime.UnlockOSThread() // soltar el hilo principal para medir con goroutines normales
	dn := pingPong(200_000, false)
	df := pingPong(20_000, true)
	fmt.Printf("  normales:        %v por vuelta\n", (dn / 200_000).Round(10*time.Nanosecond))
	fmt.Printf("  ambas fijadas:   %v por vuelta  (%.0f× más lento)\n", (df / 20_000).Round(10*time.Nanosecond), float64(df/20_000)/float64(dn/200_000))

	fmt.Println("\n== 4) una goroutine que termina SIN soltar el hilo lo destruye ==")
	fmt.Printf("  200 goroutines sucesivas que se fijan y terminan sin soltar: %d hilos del SO distintos\n", hilosDistintos(200, false))
	fmt.Printf("  200 goroutines sucesivas que se fijan y SUELTAN el hilo:     %d hilos distintos (se reutilizan)\n", hilosDistintos(200, true))
}
