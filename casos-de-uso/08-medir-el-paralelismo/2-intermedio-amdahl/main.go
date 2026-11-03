// Caso 8.2 (intermedio): la ley de Amdahl medida, no solo enunciada.
//
// Problema: una tarea tiene una parte que se puede repartir y otra que NO (leer la entrada,
// combinar resultados, escribir la salida…). Si la fracción serial es s, con k núcleos el máximo
// teórico es
//
//	speedup(k) ≤ 1 / ( s + (1−s)/k )         y, con infinitos núcleos,  ≤ 1/s
//
// Aquí fabricamos tareas con s = 0 %, 5 %, 25 % y 50 % de trabajo serial, las ejecutamos con
// 2, 4 y 8 workers y comparamos lo medido con lo predicho.
//
//	go run .
package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

// Gasta: CPU pura (xorshift), igual que en el caso 8.1.
func Gasta(iter int, semilla uint64) uint64 {
	x := semilla
	for range iter {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
	}
	return x
}

const bloques = 64

// Tarea gasta `total` iteraciones: una fracción s en un solo hilo (parte serial) y el resto
// repartido en `bloques` bloques entre k workers (parte paralela).
func Tarea(total int, s float64, k int) uint64 {
	serial := int(float64(total) * s)
	r := Gasta(serial, 1) // parte serial: nadie más puede ayudar aquí

	porBloque := (total - serial) / bloques
	parciales := make([]uint64, k)
	var wg sync.WaitGroup
	for w := range k {
		wg.Go(func() {
			// reparto estático y estrictamente igual: así la parte paralela escala bien y lo que
			// vemos es solo el efecto de la parte serial
			for b := w; b < bloques; b += k {
				parciales[w] ^= Gasta(porBloque, uint64(b)+2)
			}
		})
	}
	wg.Wait()
	for _, p := range parciales {
		r ^= p
	}
	return r
}

// Amdahl devuelve el speedup máximo teórico con fracción serial s y k núcleos.
func Amdahl(s float64, k int) float64 {
	return 1 / (s + (1-s)/float64(k))
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
	const total = 400_000_000
	fmt.Printf("GOMAXPROCS=%d. Tarea de %d M iteraciones; fracción serial s (mejor de 3).\n", runtime.GOMAXPROCS(0), total/1_000_000)
	fmt.Println("Cada celda: aceleración medida (teórica de Amdahl).")
	fmt.Println()
	fmt.Println("s serial   T(1)       k=2             k=4             k=8             techo 1/s")
	for _, s := range []float64{0, 0.05, 0.25, 0.5} {
		base := mejor(3, func() { Tarea(total, s, 1) })
		fmt.Printf("%-10s %-10v", fmt.Sprintf("%.0f %%", 100*s), base.Round(time.Millisecond))
		for _, k := range []int{2, 4, 8} {
			d := mejor(3, func() { Tarea(total, s, k) })
			fmt.Printf(" %-15s", fmt.Sprintf("%.2f× (%.2f×)", float64(base)/float64(d), Amdahl(s, k)))
		}
		if s == 0 {
			fmt.Println(" ∞")
		} else {
			fmt.Printf(" %.0f×\n", 1/s)
		}
	}
}
