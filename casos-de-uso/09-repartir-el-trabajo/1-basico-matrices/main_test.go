package main

import (
	"slices"
	"testing"
)

func TestParalelaIgualQueSecuencial(t *testing.T) {
	for _, n := range []int{1, 2, 7, 33} {
		a, b := Nueva(n), Nueva(n)
		a.Rellena(1)
		b.Rellena(2)
		want := MultiplicaSecuencial(a, b)
		for _, k := range []int{1, 2, 3, 8, 100} {
			got := MultiplicaParalela(a, b, k)
			// mismas operaciones en el mismo orden por elemento: resultado idéntico bit a bit
			if !slices.Equal(got.datos, want.datos) {
				t.Errorf("n=%d k=%d: resultado distinto", n, k)
			}
		}
	}
}

func TestIdentidad(t *testing.T) {
	const n = 20
	a, id := Nueva(n), Nueva(n)
	a.Rellena(5)
	for i := range n {
		id.datos[i*n+i] = 1
	}
	if got := MultiplicaParalela(a, id, 4); !slices.Equal(got.datos, a.datos) {
		t.Fatal("A × I debe ser A")
	}
}
