// Ejercicio 1: escalabilidad con el número de núcleos — medir, no suponer.
//
// Se reparte una cantidad FIJA de trabajo entre k goroutines (escalado "fuerte") para
// tres tipos de carga y se mide cuánto mejora el tiempo.
//
//	go run .
package main

import (
	"fmt"
	"runtime"
	"slices"
	"sync"
	"time"
)

var sinks [64]uint64 // evita que el compilador elimine el trabajo (un elemento por worker, sin compartir)

// ---- Carga 1: cálculo puro (cabe en registros; no toca memoria) ----

const iteraciones = 600_000_000

func calculo(k int) {
	var wg sync.WaitGroup
	for w := range k {
		wg.Go(func() {
			x := uint64(w)*2654435761 + 88172645463325252
			for range iteraciones / k {
				x ^= x << 13 // xorshift: dependencias entre iteraciones, sin memoria
				x ^= x >> 7
				x ^= x << 17
			}
			sinks[w] = x
		})
	}
	wg.Wait()
}

// ---- Carga 2: lectura de un array mucho mayor que la caché (limitada por el ancho de banda) ----

var datos = func() []uint64 {
	d := make([]uint64, 64<<20) // 512 MiB
	for i := range d {
		d[i] = uint64(i)
	}
	return d
}()

const pasadas = 4

func memoria(k int) {
	var wg sync.WaitGroup
	trozo := len(datos) / k
	for w := range k {
		wg.Go(func() {
			ini, fin := w*trozo, (w+1)*trozo
			var s uint64
			for range pasadas {
				for _, v := range datos[ini:fin] {
					s += v
				}
			}
			sinks[w] = s
		})
	}
	wg.Wait()
}

// ---- Carga 3: muchas asignaciones pequeñas (asignador y GC compartidos) ----

const objetos = 24_000_000

type pequeno struct {
	sig *pequeno
	v   [6]int64
}

func asignaciones(k int) {
	var wg sync.WaitGroup
	for w := range k {
		wg.Go(func() {
			var cabeza *pequeno
			for i := range objetos / k {
				cabeza = &pequeno{sig: cabeza}
				if i%64 == 0 {
					cabeza = nil // la mayor parte es basura inmediata
				}
			}
			if cabeza != nil {
				sinks[w] = uint64(cabeza.v[0])
			}
		})
	}
	wg.Wait()
}

func mejorDe(n int, f func()) time.Duration {
	mejor := time.Duration(1 << 62)
	for range n {
		ini := time.Now()
		f()
		mejor = min(mejor, time.Since(ini))
	}
	return mejor
}

func tabla(nombre string, f func(k int), ks []int) {
	fmt.Printf("\n== %s ==\n", nombre)
	fmt.Printf("%8s %12s %10s %12s\n", "workers", "tiempo", "speedup", "eficiencia")
	var base time.Duration
	var speedups []float64
	for _, k := range ks {
		d := mejorDe(3, func() { f(k) })
		if k == 1 {
			base = d
		}
		s := float64(base) / float64(d)
		speedups = append(speedups, s)
		fmt.Printf("%8d %12v %9.2fx %11.0f%%\n", k, d.Round(time.Millisecond), s, 100*s/float64(k))
	}
	// Fracción paralela implícita (Amdahl) a partir de k = NumCPU: p = (1 - 1/S) / (1 - 1/k)
	n := runtime.NumCPU()
	if i := slices.Index(ks, n); i >= 0 && speedups[i] > 1 {
		p := (1 - 1/speedups[i]) / (1 - 1/float64(n))
		fmt.Printf("  -> con %d workers: fracción paralela implícita (Amdahl) ≈ %.1f%%; speedup máximo con infinitos núcleos ≈ %.1fx\n", n, 100*p, 1/(1-p))
	}
}

func main() {
	n := runtime.NumCPU()
	fmt.Printf("NumCPU=%d GOMAXPROCS=%d\n", n, runtime.GOMAXPROCS(0))
	ks := []int{1, 2, 4, 6, n, 2 * n}
	ks = slices.Compact(ks)

	tabla("cálculo puro (xorshift, 600 M iteraciones)", calculo, ks)
	tabla("lectura de memoria (512 MiB × 4 pasadas)", memoria, ks)
	tabla("asignaciones pequeñas (24 M objetos)", asignaciones, ks)
}
