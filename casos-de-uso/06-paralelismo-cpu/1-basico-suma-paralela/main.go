// Caso 6.1 (básico): repartir un cálculo sobre un slice entre varios núcleos.
//
// Problema: sumar 10 millones de números. La suma es CPU pura (no hay esperas): la única forma de
// acelerarla es usar más núcleos a la vez (paralelismo, no solo concurrencia).
//
// Receta de paralelismo de datos:
//  1. PARTICIONAR en trozos contiguos, uno por worker.
//  2. Cada worker acumula en una variable LOCAL (nada compartido, nada que proteger).
//  3. COMBINAR los parciales al final.
//
// Y una lección importante: NO siempre compensa. Con pocos datos, lanzar goroutines cuesta más
// que el cálculo; con muchos, la memoria (no la CPU) acaba siendo el límite.
//
//	go run .
package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

func SumaSecuencial(xs []int64) (s int64) {
	for _, x := range xs {
		s += x
	}
	return
}

// SumaParalela reparte xs en k trozos contiguos.
func SumaParalela(xs []int64, k int) int64 {
	k = max(1, min(k, len(xs)))
	parciales := make([]int64, k) // cada worker escribe SOLO en su posición: sin cerrojos
	var wg sync.WaitGroup
	for i := range k {
		desde, hasta := i*len(xs)/k, (i+1)*len(xs)/k
		wg.Go(func() { parciales[i] = SumaSecuencial(xs[desde:hasta]) })
	}
	wg.Wait()
	return SumaSecuencial(parciales) // combinar
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
	fmt.Printf("GOMAXPROCS=%d. Tiempo (mejor de 7) y aceleración frente a la versión secuencial:\n\n", runtime.GOMAXPROCS(0))
	for _, n := range []int{1_000, 100_000, 10_000_000} {
		xs := make([]int64, n)
		for i := range xs {
			xs[i] = int64(i % 7)
		}
		base := mejor(7, func() { SumaSecuencial(xs) })
		fmt.Printf("n = %-10d secuencial %-10v", n, base)
		for _, k := range []int{2, 4, 8} {
			d := mejor(7, func() { SumaParalela(xs, k) })
			fmt.Printf(" | k=%d %-10v (%.1f×)", k, d, float64(base)/float64(d))
		}
		fmt.Println()
	}
}
