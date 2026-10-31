// Ejercicio 6: paralelismo de datos — map-reduce.
//
//	go run .
package main

import (
	"fmt"
	"math/rand/v2"
	"runtime"
	"time"
)

const total = 20_000_000

// generar produce `n` claves con distribución sesgada (pocas muy frecuentes) en [0, distintas).
func generar(n int, distintas uint32) []uint32 {
	r := rand.New(rand.NewPCG(1, 2))
	d := make([]uint32, n)
	for i := range d {
		x := r.Float64()
		d[i] = uint32(x * x * float64(distintas)) // sesgo hacia las claves bajas
	}
	return d
}

func contar(k uint32) (uint32, int) { return k, 1 }
func sumar(a, b int) int            { return a + b }

func mejor(n int, f func()) time.Duration {
	m := time.Duration(1 << 62)
	for range n {
		ini := time.Now()
		f()
		m = min(m, time.Since(ini))
	}
	return m
}

func escenario(titulo string, distintas uint32) {
	datos := generar(total, distintas)
	n := runtime.NumCPU()
	fmt.Printf("\n== %s (%d M elementos, hasta %d claves distintas) ==\n", titulo, total/1_000_000, distintas)

	tSeq := mejor(3, func() { Secuencial(datos, contar, sumar) })
	fmt.Printf("%-34s %10v %8s\n", "secuencial", tSeq.Round(time.Millisecond), "1.00x")

	for _, w := range []int{2, 4, n} {
		fmt.Printf("-- %d workers\n", w)
		t1 := mejor(3, func() { CompartidoConMutex(datos, w, contar, sumar) })
		t2 := mejor(3, func() { LocalYMergeSecuencial(datos, w, contar, sumar) })
		t3 := mejor(3, func() { LocalYMergeParalelo(datos, w, contar, sumar) })
		fmt.Printf("   %-31s %10v %7.2fx\n", "mapa compartido + mutex", t1.Round(time.Millisecond), float64(tSeq)/float64(t1))
		fmt.Printf("   %-31s %10v %7.2fx\n", "locales + fusión secuencial", t2.Round(time.Millisecond), float64(tSeq)/float64(t2))
		fmt.Printf("   %-31s %10v %7.2fx\n", "locales + fusión paralela", t3.Round(time.Millisecond), float64(tSeq)/float64(t3))
	}
}

func main() {
	fmt.Printf("NumCPU=%d\n", runtime.NumCPU())
	escenario("POCAS claves: la fusión es barata", 1_000)
	escenario("MUCHAS claves: la fusión pesa", 2_000_000)
}
