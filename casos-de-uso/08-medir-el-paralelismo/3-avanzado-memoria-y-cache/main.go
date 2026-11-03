// Caso 8.3 (avanzado): no todo el cálculo escala igual. Tres trabajos "paralelos" y muy distintos.
//
// Problema: el mismo reparto en k workers da aceleraciones radicalmente distintas según QUÉ
// recurso limite al trabajo:
//
//	A) CPU pura, acumulador en un registro local ........ escala casi lineal
//	B) CPU pura, pero los acumuladores de los workers
//	   están JUNTOS en un slice (misma línea de caché) ... false sharing: se estanca antes
//	C) recorrer 256 MB de memoria ........................ el ancho de banda de memoria es el techo
//
// La lección: antes de añadir núcleos hay que saber cuál es el cuello de botella real.
//
//	go run .
package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

// salto: 128 bytes (16 × uint64). Las líneas de caché del M1 son de 128 B (en x86 son de 64 B).
const salto = 16

// A: acumulador local. Cada worker trabaja con su variable en un registro.
func KernelLocal(total, k int) uint64 {
	parciales := make([]uint64, k)
	var wg sync.WaitGroup
	for w := range k {
		wg.Go(func() {
			var acc uint64
			for i := range total / k {
				acc += uint64(i) ^ 0x9e3779b97f4a7c15
			}
			parciales[w] = acc // una sola escritura al terminar
		})
	}
	wg.Wait()
	var r uint64
	for _, p := range parciales {
		r += p
	}
	return r
}

// B: mismo cálculo, pero el acumulador vive en memoria compartida y se actualiza en cada iteración.
// paso = 1 → acumuladores contiguos (misma línea de caché: false sharing).
// paso = salto → cada acumulador en su propia línea (sin false sharing).
func KernelEnSlice(total, k, paso int) uint64 {
	acum := make([]uint64, k*paso)
	var wg sync.WaitGroup
	for w := range k {
		wg.Go(func() {
			p := &acum[w*paso]
			for i := range total / k {
				*p += uint64(i) ^ 0x9e3779b97f4a7c15
			}
		})
	}
	wg.Wait()
	var r uint64
	for w := range k {
		r += acum[w*paso]
	}
	return r
}

// C: suma de un slice enorme. Casi no hay cálculo: el límite es cuántos bytes/s entrega la memoria.
func KernelMemoria(xs []uint64, k int) uint64 {
	parciales := make([]uint64, k)
	var wg sync.WaitGroup
	for w := range k {
		wg.Go(func() {
			var acc uint64
			for _, x := range xs[w*len(xs)/k : (w+1)*len(xs)/k] {
				acc += x
			}
			parciales[w] = acc
		})
	}
	wg.Wait()
	var r uint64
	for _, p := range parciales {
		r += p
	}
	return r
}

// redondea muestra ms enteros para tiempos largos y décimas de ms para los cortos.
func redondea(d time.Duration) time.Duration {
	if d < 50*time.Millisecond {
		return d.Round(100 * time.Microsecond)
	}
	return d.Round(time.Millisecond)
}

func main() {
	fmt.Printf("GOMAXPROCS=%d. Aceleración frente a 1 worker (mejor de 5, kernels intercalados).\n\n", runtime.GOMAXPROCS(0))

	const total = 800_000_000
	xs := make([]uint64, 32<<20) // 32 M × 8 B = 256 MB
	for i := range xs {
		xs[i] = uint64(i)
	}

	type kernel struct {
		nombre string
		f      func(k int)
	}
	kernels := []kernel{
		{"A  cálculo, acumulador local", func(k int) { KernelLocal(total, k) }},
		{"B1 acumuladores contiguos (false sharing)", func(k int) { KernelEnSlice(total, k, 1) }},
		{"B2 acumuladores a 128 B (sin false sharing)", func(k int) { KernelEnSlice(total, k, salto) }},
		{"C  suma de 256 MB (ancho de banda)", func(k int) { KernelMemoria(xs, k) }},
	}
	// Medimos los kernels INTERCALADOS (mejor de 5 por kernel y k): si la máquina tiene carga
	// ajena que va y viene, afecta a todos por igual en vez de falsear solo al último.
	ks := []int{1, 2, 4, 8}
	t := make([][]time.Duration, len(kernels))
	for i := range t {
		t[i] = make([]time.Duration, len(ks))
		for j := range t[i] {
			t[i][j] = time.Duration(1 << 62)
		}
	}
	for range 5 {
		for j, k := range ks {
			for i, kn := range kernels {
				t0 := time.Now()
				kn.f(k)
				t[i][j] = min(t[i][j], time.Since(t0))
			}
		}
	}
	fmt.Printf("%-46s %-14s %-14s %-14s %-14s\n", "", "k=1", "k=2", "k=4", "k=8")
	for i, kn := range kernels {
		fmt.Printf("%-46s %-14v", kn.nombre, redondea(t[i][0]))
		for j := 1; j < len(ks); j++ {
			fmt.Printf(" %-14s", fmt.Sprintf("%v (%.1f×)", redondea(t[i][j]), float64(t[i][0])/float64(t[i][j])))
		}
		fmt.Println()
	}
}
