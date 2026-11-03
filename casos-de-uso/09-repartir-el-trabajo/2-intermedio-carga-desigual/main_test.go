package main

import (
	"slices"
	"testing"
)

func TestTodasLasEstrategiasDanLaMismaImagen(t *testing.T) {
	want := make([]uint16, ancho*alto)
	Secuencial(want)
	for nombre, e := range map[string]Estrategia{
		"estática": Estatica, "cíclica": Ciclica, "dinámica 1": Dinamica(1), "dinámica 16": Dinamica(16), "dinámica 1000": Dinamica(1000),
	} {
		for _, k := range []int{1, 3, 8} {
			got := make([]uint16, ancho*alto)
			costes := e(got, k)
			if !slices.Equal(got, want) {
				t.Errorf("%s k=%d: imagen distinta", nombre, k)
			}
			if len(costes) != k {
				t.Errorf("%s: %d costes, esperado %d", nombre, len(costes), k)
			}
		}
	}
}

func TestLaCargaEsDesigual(t *testing.T) {
	// Propiedad que justifica el caso: el reparto estático queda MÁS desequilibrado que el dinámico.
	// Se mide con el coste (iteraciones), no con el reloj, así que es determinista.
	img := make([]uint16, ancho*alto)
	est := desequilibrio(Estatica(img, 8))
	din := desequilibrio(Dinamica(1)(img, 8))
	if est < 1.3 {
		t.Errorf("el reparto estático debería ser desequilibrado en Mandelbrot: %.2f", est)
	}
	if din >= est {
		t.Errorf("dinámico (%.2f) debería equilibrar mejor que estático (%.2f)", din, est)
	}
}
