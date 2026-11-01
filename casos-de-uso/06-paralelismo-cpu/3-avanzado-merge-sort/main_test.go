package main

import (
	"math/rand/v2"
	"slices"
	"testing"
)

func TestOrdenaComoLaBiblioteca(t *testing.T) {
	r := rand.New(rand.NewPCG(7, 7))
	for _, n := range []int{0, 1, 2, 3, 100, 10_001} {
		for _, umbral := range []int{0, 1, 16, 1000} {
			xs := make([]int, n)
			for i := range xs {
				xs[i] = r.IntN(50) // muchos duplicados
			}
			esperado := slices.Clone(xs)
			slices.Sort(esperado)
			OrdenarParalelo(xs, umbral)
			if !slices.Equal(xs, esperado) {
				t.Fatalf("n=%d umbral=%d: resultado distinto", n, umbral)
			}
		}
	}
}

func TestYaOrdenadoYAlReves(t *testing.T) {
	asc := make([]int, 5000)
	for i := range asc {
		asc[i] = i
	}
	desc := slices.Clone(asc)
	slices.Reverse(desc)
	for _, xs := range [][]int{slices.Clone(asc), desc} {
		OrdenarParalelo(xs, 64)
		if !slices.Equal(xs, asc) {
			t.Fatal("no ordenó")
		}
	}
}
