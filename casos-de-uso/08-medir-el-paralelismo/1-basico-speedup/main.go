// Caso 8.1 (básico): medir cuánto acelera de verdad el paralelismo, y distinguirlo de la concurrencia.
//
// Problema: tenemos 256 bloques de cálculo puro (sin E/S, sin memoria compartida). ¿Cuánto más
// rápido termina el programa con 2, 4, 8, 16 workers? Y, sobre todo: ¿qué pasa con las MISMAS
// goroutines si el runtime solo puede usar un núcleo (GOMAXPROCS=1)?
//
// Métricas que hay que saber calcular siempre que se paraleliza:
//
//	aceleración (speedup) = T(1 worker) / T(k workers)
//	eficiencia            = speedup / núcleos realmente usables   (1.0 = reparto perfecto)
//
//	go run .
package main

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// Gasta es CPU pura: una cadena de dependencias (xorshift) que ni usa memoria ni espera a nadie.
// Es el caso ideal para paralelizar, así que lo que midamos será el límite del hardware.
func Gasta(iter int, semilla uint64) uint64 {
	x := semilla
	for range iter {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
	}
	return x
}

// Ejecutar procesa nBloques con k workers. Cada worker coge el siguiente bloque libre de un
// contador atómico (reparto dinámico) y acumula en una variable LOCAL; el resultado final (XOR)
// no depende del orden, así que es el mismo para cualquier k.
func Ejecutar(nBloques, iter, k int) uint64 {
	k = max(1, k)
	var siguiente atomic.Int64
	parciales := make([]uint64, k)
	var wg sync.WaitGroup
	for w := range k {
		wg.Go(func() {
			var local uint64
			for {
				i := int(siguiente.Add(1)) - 1
				if i >= nBloques {
					break
				}
				local ^= Gasta(iter, uint64(i)+1)
			}
			parciales[w] = local
		})
	}
	wg.Wait()
	var r uint64
	for _, p := range parciales {
		r ^= p
	}
	return r
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
	const nBloques, iter = 256, 1_000_000
	nucleos := runtime.GOMAXPROCS(0)
	fmt.Printf("GOMAXPROCS=%d. %d bloques de %d iteraciones de CPU pura (mejor de 3).\n\n", nucleos, nBloques, iter)

	base := mejor(3, func() { Ejecutar(nBloques, iter, 1) })
	fmt.Println("workers   tiempo        aceleración   eficiencia")
	for _, k := range []int{1, 2, 4, 8, 16, 32} {
		d := base
		if k > 1 {
			d = mejor(3, func() { Ejecutar(nBloques, iter, k) })
		}
		s := float64(base) / float64(d)
		fmt.Printf("%-9d %-13v %-13s %.0f %%\n", k, d.Round(time.Millisecond), fmt.Sprintf("%.2f×", s), 100*s/float64(min(k, nucleos)))
	}

	// Mismas 8 goroutines, pero el runtime solo puede ejecutar UNA a la vez: hay concurrencia
	// (8 tareas en curso) pero ningún paralelismo.
	runtime.GOMAXPROCS(1)
	d := mejor(3, func() { Ejecutar(nBloques, iter, 8) })
	runtime.GOMAXPROCS(nucleos)
	fmt.Printf("\n8 workers con GOMAXPROCS=1: %v (%.2f× frente a 1 worker) → concurrencia SIN paralelismo\n",
		d.Round(time.Millisecond), float64(base)/float64(d))
}
