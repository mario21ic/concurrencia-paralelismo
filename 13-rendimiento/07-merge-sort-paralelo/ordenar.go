// Package main: ordenación por mezcla (merge sort) secuencial y paralela.
package main

import (
	"sort"
	"sync"
	"sync/atomic"
)

const umbralInsercion = 24

// insercion ordena trozos pequeños (más rápido que recursar hasta 1 elemento).
func insercion(a []int) {
	for i := 1; i < len(a); i++ {
		v := a[i]
		j := i - 1
		for j >= 0 && a[j] > v {
			a[j+1] = a[j]
			j--
		}
		a[j+1] = v
	}
}

// mezclar combina dos secuencias ordenadas x e y en dst (len(dst) == len(x)+len(y)).
func mezclar(dst, x, y []int) {
	i, j, k := 0, 0, 0
	for i < len(x) && j < len(y) {
		if x[i] <= y[j] {
			dst[k] = x[i]
			i++
		} else {
			dst[k] = y[j]
			j++
		}
		k++
	}
	k += copy(dst[k:], x[i:])
	copy(dst[k:], y[j:])
}

// OrdenarSecuencial: merge sort clásico con búfer auxiliar `tmp` (misma longitud que a).
func OrdenarSecuencial(a, tmp []int) {
	if len(a) <= umbralInsercion {
		insercion(a)
		return
	}
	m := len(a) / 2
	OrdenarSecuencial(a[:m], tmp[:m])
	OrdenarSecuencial(a[m:], tmp[m:])
	mezclar(tmp, a[:m], a[m:])
	copy(a, tmp)
}

// Config controla la versión paralela.
type Config struct {
	Umbral       int           // por debajo de este tamaño, se ordena en secuencial
	MezclaPar    bool          // true: la mezcla también se paraleliza
	UmbralMezcla int           // tamaño mínimo para dividir una mezcla
	Goroutines   *atomic.Int64 // cuenta las goroutines lanzadas (opcional)
}

func (c Config) lanzada() {
	if c.Goroutines != nil {
		c.Goroutines.Add(1)
	}
}

// OrdenarParalelo ordena las dos mitades en paralelo; por debajo de Umbral recurre a la versión
// secuencial. Las dos mitades trabajan sobre rangos DISJUNTOS de a y tmp: no hay carreras.
func OrdenarParalelo(a, tmp []int, c Config) {
	if len(a) <= c.Umbral || len(a) <= umbralInsercion {
		OrdenarSecuencial(a, tmp)
		return
	}
	m := len(a) / 2
	var wg sync.WaitGroup
	c.lanzada()
	wg.Go(func() { OrdenarParalelo(a[:m], tmp[:m], c) })
	OrdenarParalelo(a[m:], tmp[m:], c)
	wg.Wait()

	if c.MezclaPar {
		mezclarParalelo(tmp, a[:m], a[m:], c)
		copiarParalelo(a, tmp, c)
	} else {
		mezclar(tmp, a[:m], a[m:])
		copy(a, tmp)
	}
}

// mezclarParalelo divide la mezcla: toma la mediana del array mayor, busca su posición en el otro
// con búsqueda binaria y mezcla en paralelo las dos partes (que son independientes).
func mezclarParalelo(dst, x, y []int, c Config) {
	if len(x)+len(y) <= max(c.UmbralMezcla, 1) {
		mezclar(dst, x, y)
		return
	}
	if len(x) < len(y) {
		x, y = y, x // x es el mayor (el resultado no depende del orden de los argumentos)
	}
	if len(y) == 0 {
		copy(dst, x)
		return
	}
	mx := len(x) / 2
	pivote := x[mx]
	my := sort.SearchInts(y, pivote) // primer índice con y[i] >= pivote
	dst[mx+my] = pivote

	var wg sync.WaitGroup
	c.lanzada()
	wg.Go(func() { mezclarParalelo(dst[:mx+my], x[:mx], y[:my], c) })
	mezclarParalelo(dst[mx+my+1:], x[mx+1:], y[my:], c)
	wg.Wait()
}

// copiarParalelo copia dst←src por trozos (la copia de n elementos también es un cuello de botella).
func copiarParalelo(dst, src []int, c Config) {
	if len(dst) <= max(c.UmbralMezcla, 1) {
		copy(dst, src)
		return
	}
	m := len(dst) / 2
	var wg sync.WaitGroup
	c.lanzada()
	wg.Go(func() { copiarParalelo(dst[:m], src[:m], c) })
	copiarParalelo(dst[m:], src[m:], c)
	wg.Wait()
}
