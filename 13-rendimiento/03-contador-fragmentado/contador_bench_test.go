package contador

import (
	"sync/atomic"
	"testing"
)

func bench(b *testing.B, c Contador) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			c.Sumar(1)
		}
	})
}

func BenchmarkAtomico(b *testing.B)               { bench(b, &Atomico{}) }
func BenchmarkMutex(b *testing.B)                 { bench(b, &ConMutex{}) }
func BenchmarkFragmentadoSinRelleno(b *testing.B) { bench(b, &FragmentadoSinRelleno{}) }
func BenchmarkFragmentadoConRelleno(b *testing.B) { bench(b, &Fragmentado{}) }

// Coste de LEER el total: el precio del fragmentado.
func BenchmarkLecturaAtomico(b *testing.B) {
	c := &Atomico{}
	for b.Loop() {
		c.Total()
	}
}

func BenchmarkLecturaFragmentado(b *testing.B) {
	c := &Fragmentado{}
	for b.Loop() {
		c.Total()
	}
}

// Fragmento AFÍN: cada goroutine del benchmark recibe un índice fijo.
func BenchmarkFragmentadoAfin(b *testing.B) {
	c := &Fragmentado{}
	var siguiente atomic.Int64
	b.RunParallel(func(pb *testing.PB) {
		id := int(siguiente.Add(1))
		for pb.Next() {
			c.SumarEn(id, 1)
		}
	})
}

// Acumulación LOCAL: cada goroutine cuenta en una variable propia y suma una vez al final.
func BenchmarkAcumulacionLocal(b *testing.B) {
	var total Atomico
	b.RunParallel(func(pb *testing.PB) {
		var local int64
		for pb.Next() {
			local++
		}
		total.Sumar(local)
	})
}
