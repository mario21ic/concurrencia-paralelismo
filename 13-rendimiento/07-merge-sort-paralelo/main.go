// Ejercicio 7: ordenación por mezcla en paralelo.
//
//	go run .
package main

import (
	"fmt"
	"math/rand/v2"
	"runtime"
	"slices"
	"sync/atomic"
	"time"
)

const n = 8_000_000

func datos() []int {
	r := rand.New(rand.NewPCG(7, 8))
	a := make([]int, n)
	for i := range a {
		a[i] = r.IntN(1 << 30)
	}
	return a
}

// medir ordena una copia fresca, comprueba el resultado y devuelve el mejor tiempo de 3.
func medir(original []int, ordenar func(a []int)) time.Duration {
	mejor := time.Duration(1 << 62)
	a := make([]int, len(original))
	for range 3 {
		copy(a, original)
		ini := time.Now()
		ordenar(a)
		mejor = min(mejor, time.Since(ini))
		if !slices.IsSorted(a) {
			panic("resultado sin ordenar")
		}
	}
	return mejor
}

func main() {
	orig := datos()
	fmt.Printf("NumCPU=%d, ordenando %d M enteros aleatorios\n\n", runtime.NumCPU(), n/1_000_000)

	tStd := medir(orig, func(a []int) { slices.Sort(a) })
	tSeq := medir(orig, func(a []int) { OrdenarSecuencial(a, make([]int, len(a))) })
	fmt.Printf("%-52s %10v %8s\n", "slices.Sort (pdqsort de la biblioteca estándar)", tStd.Round(time.Millisecond), "")
	fmt.Printf("%-52s %10v %8s\n", "merge sort secuencial (referencia)", tSeq.Round(time.Millisecond), "1.00x")

	var g atomic.Int64
	probar := func(nombre string, c Config) {
		g.Store(0)
		c.Goroutines = &g
		t := medir(orig, func(a []int) { OrdenarParalelo(a, make([]int, len(a)), c) })
		fmt.Printf("%-52s %10v %7.2fx   goroutines/orden: %d\n", nombre, t.Round(time.Millisecond), float64(tSeq)/float64(t), g.Load()/3)
	}

	fmt.Println("\n-- el umbral (tamaño por debajo del cual no se crean más goroutines) --")
	for _, u := range []int{32, 1_000, 16_000, 256_000, 1_000_000, 4_000_000} {
		probar(fmt.Sprintf("paralelo, mezcla secuencial, umbral=%d", u), Config{Umbral: u})
	}

	fmt.Println("\n-- paralelizando también la mezcla (el paso final es secuencial y limita el speedup) --")
	for _, u := range []int{16_000, 256_000} {
		probar(fmt.Sprintf("paralelo + mezcla paralela, umbral=%d", u), Config{Umbral: u, MezclaPar: true, UmbralMezcla: 16_000})
	}
}
