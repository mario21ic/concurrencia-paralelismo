// Caso 6.3 (avanzado): divide y vencerás en paralelo — ordenación por mezcla con umbral.
//
// Problema: ordenar 4 millones de enteros usando todos los núcleos.
// Idea: ordenar las dos mitades EN PARALELO (cada una en su goroutine) y mezclar. Recursivamente.
//
// Dos decisiones que importan:
//  1. UMBRAL: por debajo de cierto tamaño NO se lanzan goroutines (coordinar cuesta más que ordenar
//     un trozo pequeño). Sin umbral se crearían millones de goroutines.
//  2. La mezcla final es SECUENCIAL: es la parte «no paralelizable» y limita el speedup (Amdahl).
//
// Referencia honesta: slices.Sort (pdqsort, secuencial) es muy buena; hay que medir antes de paralelizar.
//
//	go run .
package main

import (
	"fmt"
	"math/rand/v2"
	"runtime"
	"slices"
	"sync"
	"time"
)

func mezclar(dst, a, b []int) {
	i, j, k := 0, 0, 0
	for i < len(a) && j < len(b) {
		if a[i] <= b[j] {
			dst[k] = a[i]
			i++
		} else {
			dst[k] = b[j]
			j++
		}
		k++
	}
	k += copy(dst[k:], a[i:])
	copy(dst[k:], b[j:])
}

// casoBase: por debajo de este tamaño se ordena directamente (insertar/pdqsort es más barato que seguir
// dividiendo). Es independiente del umbral de PARALELISMO.
const casoBase = 32

// ordenar ordena xs usando tmp como espacio auxiliar (mismo tamaño). Las dos mitades se ordenan
// recursivamente; si len(xs) > umbral la izquierda va en otra goroutine mientras esta hace la derecha.
func ordenar(xs, tmp []int, umbral int) {
	if len(xs) <= casoBase {
		slices.Sort(xs)
		return
	}
	m := len(xs) / 2
	if len(xs) > umbral {
		var wg sync.WaitGroup
		wg.Go(func() { ordenar(xs[:m], tmp[:m], umbral) }) // mitad izquierda en paralelo
		ordenar(xs[m:], tmp[m:], umbral)                   // esta goroutine hace la derecha (no se desperdicia)
		wg.Wait()
	} else {
		ordenar(xs[:m], tmp[:m], umbral)
		ordenar(xs[m:], tmp[m:], umbral)
	}
	mezclar(tmp, xs[:m], xs[m:])
	copy(xs, tmp)
}

// OrdenarParalelo ordena xs in situ. Solo los subproblemas MAYORES que `umbral` se reparten entre
// goroutines. umbral <= 0 desactiva el paralelismo (mezcla totalmente secuencial).
func OrdenarParalelo(xs []int, umbral int) {
	if umbral <= 0 {
		umbral = len(xs) + 1
	}
	ordenar(xs, make([]int, len(xs)), umbral)
}

func datos(n int) []int {
	r := rand.New(rand.NewPCG(1, 2))
	xs := make([]int, n)
	for i := range xs {
		xs[i] = r.IntN(1 << 30)
	}
	return xs
}

func medir(n int, f func([]int)) time.Duration {
	m := time.Duration(1 << 62)
	for range 5 {
		xs := datos(n)
		t0 := time.Now()
		f(xs)
		m = min(m, time.Since(t0))
	}
	return m
}

func main() {
	const n = 4_000_000
	fmt.Printf("ordenar %d enteros, GOMAXPROCS=%d (mejor de 5)\n", n, runtime.GOMAXPROCS(0))
	ref := medir(n, func(xs []int) { slices.Sort(xs) })
	fmt.Printf("  slices.Sort (secuencial, referencia)   %v\n", ref)
	sec := medir(n, func(xs []int) { OrdenarParalelo(xs, 0) })
	fmt.Printf("  mezcla SECUENCIAL (propia, sin goroutines) %v\n", sec)
	for _, umbral := range []int{8, 1024, 16_384, 131_072, 1_000_000} {
		d := medir(n, func(xs []int) { OrdenarParalelo(xs, umbral) })
		fmt.Printf("  mezcla paralela, umbral %-9d       %-10v (%.1f× vs mezcla secuencial)\n", umbral, d, float64(sec)/float64(d))
	}
}
