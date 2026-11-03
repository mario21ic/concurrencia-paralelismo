// Caso 9.2 (intermedio): cuando las partes NO cuestan lo mismo, cómo repartes importa más que cuántos
// workers pongas.
//
// Problema: pintar el conjunto de Mandelbrot (1024×1024). Cada píxel cuesta entre 1 y 800 iteraciones:
// las filas del centro (dentro del conjunto) son MUCHO más caras que las de los bordes. Si das a cada
// worker un bloque contiguo de filas, el que cae en el centro tarda varias veces más que los demás y
// el resto se queda esperando: el tiempo total lo marca el MÁS LENTO.
//
// Cuatro estrategias de reparto de las mismas filas entre k = 8 workers:
//
//	estático      bloques contiguos de n/k filas ............ simple, pero desequilibrado
//	cíclico       la fila i va al worker i % k .............. reparte las zonas caras, sin coordinación
//	dinámico (1)  contador atómico, de fila en fila ......... equilibrio máximo, más tráfico en el contador
//	dinámico (16) contador atómico, de 16 en 16 filas ....... equilibrio casi igual con ~16× menos tráfico
//
// Métrica de equilibrio: cuánto trabajó cada worker (máximo / media; 1.00 = reparto perfecto).
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

const (
	ancho, alto = 1024, 1024
	maxIter     = 800
)

// iteraciones devuelve cuántas iteraciones tarda el punto (cr, ci) en escapar (o maxIter si no escapa).
func iteraciones(cr, ci float64) int {
	var zr, zi float64
	for n := range maxIter {
		zr2, zi2 := zr*zr, zi*zi
		if zr2+zi2 > 4 {
			return n
		}
		zi = 2*zr*zi + ci
		zr = zr2 - zi2 + cr
	}
	return maxIter
}

// Fila calcula la fila y de la imagen y devuelve el coste (suma de iteraciones).
func Fila(img []uint16, y int) (coste int) {
	ci := -1.5 + 3*float64(y)/alto
	for x := range ancho {
		it := iteraciones(-2+3.5*float64(x)/ancho, ci)
		img[y*ancho+x] = uint16(it)
		coste += it
	}
	return
}

// Estrategia reparte las filas entre k workers y devuelve el coste que asumió cada uno.
type Estrategia func(img []uint16, k int) []int

func Secuencial(img []uint16) {
	for y := range alto {
		Fila(img, y)
	}
}

func Estatica(img []uint16, k int) []int {
	costes := make([]int, k)
	var wg sync.WaitGroup
	for w := range k {
		wg.Go(func() {
			for y := w * alto / k; y < (w+1)*alto/k; y++ {
				costes[w] += Fila(img, y)
			}
		})
	}
	wg.Wait()
	return costes
}

func Ciclica(img []uint16, k int) []int {
	costes := make([]int, k)
	var wg sync.WaitGroup
	for w := range k {
		wg.Go(func() {
			for y := w; y < alto; y += k {
				costes[w] += Fila(img, y)
			}
		})
	}
	wg.Wait()
	return costes
}

// Dinamica devuelve una estrategia en la que cada worker toma el siguiente lote de `lote` filas
// libres de un contador atómico: el que termina antes, coge más.
func Dinamica(lote int) Estrategia {
	return func(img []uint16, k int) []int {
		costes := make([]int, k)
		var sig atomic.Int64
		var wg sync.WaitGroup
		for w := range k {
			wg.Go(func() {
				for {
					ini := int(sig.Add(int64(lote))) - lote
					if ini >= alto {
						return
					}
					for y := ini; y < min(ini+lote, alto); y++ {
						costes[w] += Fila(img, y)
					}
				}
			})
		}
		wg.Wait()
		return costes
	}
}

func desequilibrio(costes []int) float64 {
	var suma, mx int
	for _, c := range costes {
		suma += c
		mx = max(mx, c)
	}
	return float64(mx) / (float64(suma) / float64(len(costes)))
}

func main() {
	const k = 8
	fmt.Printf("GOMAXPROCS=%d, k=%d workers. Mandelbrot %d×%d (mejor de 3).\n\n", runtime.GOMAXPROCS(0), k, ancho, alto)

	img := make([]uint16, ancho*alto)
	base := time.Duration(1 << 62)
	for range 3 {
		t0 := time.Now()
		Secuencial(img)
		base = min(base, time.Since(t0))
	}
	fmt.Printf("%-16s %-10v 1.00×\n", "secuencial", base.Round(time.Millisecond))

	fmt.Printf("\n%-16s %-10s %-11s %s\n", "estrategia", "tiempo", "aceleración", "desequilibrio (máx/media)")
	for _, e := range []struct {
		nombre string
		f      Estrategia
	}{
		{"estático", Estatica},
		{"cíclico", Ciclica},
		{"dinámico (1)", Dinamica(1)},
		{"dinámico (16)", Dinamica(16)},
	} {
		d := time.Duration(1 << 62)
		var costes []int
		for range 3 {
			t0 := time.Now()
			costes = e.f(img, k)
			d = min(d, time.Since(t0))
		}
		fmt.Printf("%-16s %-10v %-11s %.2f\n", e.nombre, d.Round(time.Millisecond), fmt.Sprintf("%.2f×", float64(base)/float64(d)), desequilibrio(costes))
	}
}
