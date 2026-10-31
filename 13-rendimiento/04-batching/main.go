// Ejercicio 4: batching — amortizar costes fijos.
//
// Cada comunicación o cada lock tiene un coste fijo (~100 ns un envío por channel, ~15-100 ns un
// lock disputado). Si el trabajo por elemento es pequeño, ese coste domina. Agrupando N
// elementos en un lote, el coste fijo se reparte entre N.
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

var elementos = 8_000_000 // variable (no constante) para que los tests usen un tamaño pequeño

func mejor(n int, f func() time.Duration) time.Duration {
	m := time.Duration(1 << 62)
	for range n {
		m = min(m, f())
	}
	return m
}

// ---- Parte 1: un productor y un consumidor por channel ----

// porLotes envía `elementos` enteros agrupados en lotes de `tam`.
// reciclar=true devuelve los lotes vacíos al productor por otro channel (cero asignaciones en régimen).
func porLotes(tam int, reciclar bool) time.Duration {
	lotes := make(chan []int, 64)
	libres := make(chan []int, 64)
	var suma atomic.Int64

	ini := time.Now()
	var wg sync.WaitGroup
	wg.Go(func() { // productor
		defer close(lotes)
		lote := make([]int, 0, tam)
		for i := range elementos {
			lote = append(lote, i)
			if len(lote) == tam {
				lotes <- lote
				if reciclar {
					select {
					case lote = <-libres:
						lote = lote[:0]
					default:
						lote = make([]int, 0, tam)
					}
				} else {
					lote = make([]int, 0, tam)
				}
			}
		}
		if len(lote) > 0 {
			lotes <- lote
		}
	})
	wg.Go(func() { // consumidor
		var s int
		for lote := range lotes {
			for _, v := range lote {
				s += v
			}
			if reciclar {
				select {
				case libres <- lote:
				default:
				}
			}
		}
		suma.Store(int64(s))
	})
	wg.Wait()
	d := time.Since(ini)
	if suma.Load() != int64(elementos)*int64(elementos-1)/2 {
		panic("suma incorrecta")
	}
	return d
}

// ---- Parte 2: muchos workers actualizando un total protegido por un mutex ----

func conMutex(workers, tam int) time.Duration {
	var mu sync.Mutex
	var total int64
	ini := time.Now()
	var wg sync.WaitGroup
	por := elementos / workers
	for range workers {
		wg.Go(func() {
			var local int64
			for i := range por {
				local++ // "procesar" un elemento
				if (i+1)%tam == 0 {
					mu.Lock() // UN lock por lote en vez de uno por elemento
					total += local
					mu.Unlock()
					local = 0
				}
			}
			if local != 0 {
				mu.Lock()
				total += local
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	d := time.Since(ini)
	if total != int64(workers*por) {
		panic("total incorrecto")
	}
	return d
}

func main() {
	fmt.Printf("NumCPU=%d, %d M elementos\n", runtime.NumCPU(), elementos/1_000_000)

	fmt.Println("\n== 1) un productor → un consumidor por channel (lotes de N elementos) ==")
	fmt.Printf("%8s %14s %14s %14s %14s\n", "lote", "ns/elemento", "vs. lote 1", "reciclando", "vs. lote 1")
	var base, baseR time.Duration
	for _, tam := range []int{1, 2, 4, 8, 16, 64, 256, 1024} {
		d := mejor(3, func() time.Duration { return porLotes(tam, false) })
		r := mejor(3, func() time.Duration { return porLotes(tam, true) })
		if tam == 1 {
			base, baseR = d, r
		}
		fmt.Printf("%8d %14.1f %13.1fx %14.1f %13.1fx\n", tam,
			float64(d)/float64(elementos), float64(base)/float64(d), float64(r)/float64(elementos), float64(baseR)/float64(r))
	}

	w := runtime.NumCPU()
	fmt.Printf("\n== 2) %d workers acumulando en un total con mutex (lock cada N elementos) ==\n", w)
	fmt.Printf("%8s %14s %14s\n", "lote", "ns/elemento", "vs. lote 1")
	var base2 time.Duration
	for _, tam := range []int{1, 4, 16, 64, 1024} {
		d := mejor(3, func() time.Duration { return conMutex(w, tam) })
		if tam == 1 {
			base2 = d
		}
		fmt.Printf("%8d %14.2f %13.1fx\n", tam, float64(d)/float64(elementos), float64(base2)/float64(d))
	}
}
