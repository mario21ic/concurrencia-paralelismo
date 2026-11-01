// Caso 6.2 (intermedio): contar palabras en paralelo — «map» local y «reduce» (fusión) al final.
//
// Problema: contar la frecuencia de cada palabra en un texto enorme. La tentación es un mapa compartido
// protegido por un mutex. Pero así los workers se pasan el tiempo ESPERÁNDOSE: es CONCURRENTE pero no
// aprovecha los núcleos (a veces es incluso más lento que el secuencial).
//
// Patrón correcto (MapReduce en miniatura):
//  1. MAP:    cada worker cuenta SU trozo en un mapa PROPIO.
//  2. REDUCE: se fusionan los mapas parciales en uno.
//
// Regla: la mejor sincronización es la que no existe — estado local + combinar al final.
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

func Secuencial(palabras []string) map[string]int {
	m := map[string]int{}
	for _, p := range palabras {
		m[p]++
	}
	return m
}

// MutexGlobal: k workers, UN mapa compartido, UN cerrojo. Correcto pero lento por contención.
func MutexGlobal(palabras []string, k int) map[string]int {
	m := map[string]int{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for i := range k {
		trozo := palabras[i*len(palabras)/k : (i+1)*len(palabras)/k]
		wg.Go(func() {
			for _, p := range trozo {
				mu.Lock()
				m[p]++
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	return m
}

// LocalYFusion: cada worker cuenta en su mapa local; al final se fusionan.
func LocalYFusion(palabras []string, k int) map[string]int {
	parciales := make([]map[string]int, k)
	var wg sync.WaitGroup
	for i := range k {
		trozo := palabras[i*len(palabras)/k : (i+1)*len(palabras)/k]
		wg.Go(func() { parciales[i] = Secuencial(trozo) }) // sin cerrojos
	}
	wg.Wait()
	total := parciales[0]
	for _, p := range parciales[1:] { // REDUCE (secuencial: es barato frente al map)
		for pal, n := range p {
			total[pal] += n
		}
	}
	return total
}

func corpus(n, vocabulario int) []string {
	r := rand.New(rand.NewPCG(1, 2))
	voc := make([]string, vocabulario)
	for i := range voc {
		voc[i] = fmt.Sprintf("palabra%d", i)
	}
	out := make([]string, n)
	for i := range out {
		out[i] = voc[r.IntN(vocabulario)]
	}
	return out
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
	k := runtime.GOMAXPROCS(0)
	palabras := corpus(4_000_000, 5000)
	fmt.Printf("%d palabras, vocabulario de 5000, %d workers (mejor de 5)\n", len(palabras), k)
	base := mejor(5, func() { Secuencial(palabras) })
	fmt.Printf("  secuencial            %-10v\n", base)
	d := mejor(5, func() { MutexGlobal(palabras, k) })
	fmt.Printf("  mapa compartido+mutex %-10v (%.1f× el secuencial)\n", d, float64(d)/float64(base))
	d = mejor(5, func() { LocalYFusion(palabras, k) })
	fmt.Printf("  local + fusión        %-10v (%.1f× más rápido que el secuencial)\n", d, float64(base)/float64(d))
}
