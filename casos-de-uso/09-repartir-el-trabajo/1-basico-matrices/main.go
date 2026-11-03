// Caso 9.1 (básico): descomposición de dominio — repartir el resultado por filas.
//
// Problema: multiplicar dos matrices de 768×768 (C = A × B). Cada elemento de C se calcula de forma
// INDEPENDIENTE de los demás, así que podemos repartir las FILAS de C entre workers sin ningún
// cerrojo: cada worker lee A y B (solo lectura, compartidas sin riesgo) y escribe SOLO en sus filas.
//
// Regla de oro del paralelismo de datos: si el reparto es por la SALIDA (cada worker escribe en un
// rango propio), no hay carreras y no hace falta sincronizar nada más que el "esperar al final".
//
//	go run .
package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

// Matriz es un bloque contiguo en orden de filas (fila i = datos[i*n : (i+1)*n]).
type Matriz struct {
	n     int
	datos []float64
}

func Nueva(n int) *Matriz { return &Matriz{n: n, datos: make([]float64, n*n)} }

func (m *Matriz) fila(i int) []float64 { return m.datos[i*m.n : (i+1)*m.n] }

// Rellena con valores deterministas (para comparar resultados entre versiones).
func (m *Matriz) Rellena(semilla int) {
	for i := range m.datos {
		m.datos[i] = float64((i*31+semilla)%17) / 8
	}
}

// filas calcula las filas [desde, hasta) de c = a × b. El orden de bucles i-k-j recorre b y c por
// filas (acceso secuencial a memoria): es mucho más amable con la caché que i-j-k.
func filas(c, a, b *Matriz, desde, hasta int) {
	n := a.n
	for i := desde; i < hasta; i++ {
		ci := c.fila(i)
		clear(ci)
		for k := range n {
			aik := a.datos[i*n+k]
			bk := b.fila(k)
			for j := range n {
				ci[j] += aik * bk[j]
			}
		}
	}
}

func MultiplicaSecuencial(a, b *Matriz) *Matriz {
	c := Nueva(a.n)
	filas(c, a, b, 0, a.n)
	return c
}

// MultiplicaParalela reparte las filas de C en k bandas contiguas.
func MultiplicaParalela(a, b *Matriz, k int) *Matriz {
	k = max(1, min(k, a.n))
	c := Nueva(a.n)
	var wg sync.WaitGroup
	for w := range k {
		desde, hasta := w*a.n/k, (w+1)*a.n/k
		wg.Go(func() { filas(c, a, b, desde, hasta) })
	}
	wg.Wait()
	return c
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
	const n = 768
	a, b := Nueva(n), Nueva(n)
	a.Rellena(1)
	b.Rellena(2)
	fmt.Printf("GOMAXPROCS=%d. C = A×B con matrices %d×%d (≈ %.1f GFLOP), mejor de 3.\n\n", runtime.GOMAXPROCS(0), n, n, 2*float64(n)*n*n/1e9)

	base := mejor(3, func() { MultiplicaSecuencial(a, b) })
	fmt.Printf("secuencial          %-10v 1.00×   %.2f GFLOP/s\n", base.Round(time.Millisecond), 2*float64(n)*n*n/1e9/base.Seconds())
	for _, k := range []int{2, 4, 8, 16} {
		d := mejor(3, func() { MultiplicaParalela(a, b, k) })
		fmt.Printf("paralela k=%-2d        %-10v %.2f×   %.2f GFLOP/s\n", k, d.Round(time.Millisecond), float64(base)/float64(d), 2*float64(n)*n*n/1e9/d.Seconds())
	}
}
