// Caso 10.1 (básico): simulación Monte Carlo — el paralelismo "vergonzosamente fácil", con una trampa.
//
// Problema: estimar π lanzando puntos al azar en el cuadrado [0,1)² y contando cuántos caen dentro del
// cuarto de círculo (proporción = π/4). Cada muestra es independiente de las demás: perfecto para
// repartir. La trampa es el GENERADOR de números aleatorios:
//
//	compartido + mutex ... un único *rand.Rand protegido: los 8 workers hacen cola en el cerrojo
//	global (math/rand/v2) funciones de nivel superior: seguras y sin cerrojo (estado por hilo)
//	local por worker ..... un PCG propio por goroutine: sin compartir nada, y REPRODUCIBLE
//	                       (misma semilla + mismo k = mismo resultado)
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

// reparte devuelve cuántas muestras le tocan al worker w (el resto de la división va a los primeros).
func reparte(muestras, k, w int) int {
	n := muestras / k
	if w < muestras%k {
		n++
	}
	return n
}

// cuenta lanza n puntos con el generador f y devuelve cuántos caen en el cuarto de círculo.
func cuenta(n int, f func() float64) (dentro int) {
	for range n {
		x, y := f(), f()
		if x*x+y*y < 1 {
			dentro++
		}
	}
	return
}

func pi(dentro, muestras int) float64 { return 4 * float64(dentro) / float64(muestras) }

// ejecuta lanza k workers; gen(w) devuelve el generador que usará el worker w.
func ejecuta(muestras, k int, gen func(w int) func() float64) float64 {
	dentro := make([]int, k)
	var wg sync.WaitGroup
	for w := range k {
		wg.Go(func() { dentro[w] = cuenta(reparte(muestras, k, w), gen(w)) })
	}
	wg.Wait()
	total := 0
	for _, d := range dentro {
		total += d
	}
	return pi(total, muestras)
}

// PiCompartidoConMutex: un solo generador para todos, protegido con un Mutex.
func PiCompartidoConMutex(muestras, k int, semilla uint64) float64 {
	var mu sync.Mutex
	r := rand.New(rand.NewPCG(semilla, 0))
	return ejecuta(muestras, k, func(int) func() float64 {
		return func() float64 {
			mu.Lock()
			defer mu.Unlock()
			return r.Float64()
		}
	})
}

// PiGlobal: las funciones de nivel superior de math/rand/v2 son seguras entre goroutines y no usan
// un cerrojo global (cada hilo tiene su estado ChaCha8). No es reproducible: no se puede fijar semilla.
func PiGlobal(muestras, k int) float64 {
	return ejecuta(muestras, k, func(int) func() float64 { return rand.Float64 })
}

// PiLocal: un PCG por worker, con flujo distinto (segundo parámetro = w). Sin compartir ni cerrojos.
func PiLocal(muestras, k int, semilla uint64) float64 {
	return ejecuta(muestras, k, func(w int) func() float64 {
		return rand.New(rand.NewPCG(semilla, uint64(w))).Float64
	})
}

func mejor(veces int, f func()) time.Duration {
	m := time.Duration(1 << 62)
	for range veces {
		t0 := time.Now()
		f()
		m = min(m, time.Since(t0))
	}
	return m
}

func main() {
	const muestras = 20_000_000
	fmt.Printf("GOMAXPROCS=%d. π con %d M muestras (mejor de 3).\n\n", runtime.GOMAXPROCS(0), muestras/1_000_000)
	fmt.Printf("%-32s %-14s %-14s %s\n", "generador", "k=1", "k=8", "π estimado (k=8)")
	var est float64
	for _, c := range []struct {
		nombre string
		f      func(k int) float64
	}{
		{"compartido + Mutex", func(k int) float64 { return PiCompartidoConMutex(muestras, k, 42) }},
		{"global math/rand/v2", func(k int) float64 { return PiGlobal(muestras, k) }},
		{"local por worker (PCG)", func(k int) float64 { return PiLocal(muestras, k, 42) }},
	} {
		d1 := mejor(3, func() { c.f(1) })
		d8 := mejor(3, func() { est = c.f(8) })
		fmt.Printf("%-32s %-14v %-14s %.5f\n", c.nombre, d1.Round(time.Millisecond), fmt.Sprintf("%v (%.1f×)", d8.Round(time.Millisecond), float64(d1)/float64(d8)), est)
	}
	fmt.Printf("\nReproducibilidad con generador local, semilla 42 y k=8: %.8f / %.8f\n", PiLocal(muestras, 8, 42), PiLocal(muestras, 8, 42))
}
