package main

import (
	"math"
	"testing"
)

func TestRepartoSumaTodasLasMuestras(t *testing.T) {
	for _, n := range []int{0, 1, 7, 1000, 1001} {
		for _, k := range []int{1, 3, 8} {
			total := 0
			for w := range k {
				total += reparte(n, k, w)
			}
			if total != n {
				t.Errorf("n=%d k=%d: %d muestras repartidas", n, k, total)
			}
		}
	}
}

func TestPiAproximado(t *testing.T) {
	const muestras = 2_000_000 // error típico ≈ 1.6/√n ≈ 0.001: la tolerancia 0.02 es muy holgada
	for nombre, got := range map[string]float64{
		"mutex":  PiCompartidoConMutex(200_000, 4, 1), // menos muestras: es el lento
		"global": PiGlobal(muestras, 4),
		"local":  PiLocal(muestras, 4, 1),
	} {
		if math.Abs(got-math.Pi) > 0.02 {
			t.Errorf("%s: π ≈ %v", nombre, got)
		}
	}
}

func TestLocalEsReproducible(t *testing.T) {
	a, b := PiLocal(500_000, 8, 7), PiLocal(500_000, 8, 7)
	if a != b {
		t.Fatalf("misma semilla y k deben dar el mismo resultado: %v vs %v", a, b)
	}
	if PiLocal(500_000, 8, 7) == PiLocal(500_000, 8, 8) {
		t.Fatal("semillas distintas deberían dar resultados distintos")
	}
}
