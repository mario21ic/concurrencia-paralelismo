package main

import (
	"math"
	"runtime/metrics"
	"testing"
)

func hist(counts ...uint64) *metrics.Float64Histogram {
	// cubos: [0,1) [1,2) [2,4) [4,+Inf)
	return &metrics.Float64Histogram{Counts: counts, Buckets: []float64{0, 1, 2, 4, math.Inf(1)}}
}

func TestCuantil(t *testing.T) {
	h := hist(50, 30, 15, 5) // 100 observaciones
	casos := []struct {
		q    float64
		want float64
	}{{0.50, 1}, {0.80, 2}, {0.95, 4}, {0.99, 4}, {1.0, 4}}
	for _, c := range casos {
		if got := Cuantil(h, c.q); got != c.want {
			t.Errorf("Cuantil(%v) = %v, want %v", c.q, got, c.want)
		}
	}
	if Cuantil(hist(0, 0, 0, 0), 0.5) != 0 {
		t.Error("histograma vacío debe dar 0")
	}
}

func TestRestarHistograma(t *testing.T) {
	d := RestarHistograma(hist(10, 20, 30, 40), hist(5, 5, 5, 5))
	want := []uint64{5, 15, 25, 35}
	for i, c := range d.Counts {
		if c != want[i] {
			t.Fatalf("Counts = %v, want %v", d.Counts, want)
		}
	}
	if Observaciones(d) != 80 {
		t.Fatalf("Observaciones = %d", Observaciones(d))
	}
}

func TestLeerDevuelveDatosCoherentes(t *testing.T) {
	f := Leer()
	if f.Goroutines == 0 || f.MaxProcs == 0 || f.Latencias == nil {
		t.Fatalf("foto incompleta: %+v", f)
	}
	if f.Running+f.Runnable+f.Waiting > f.Goroutines+2 { // pueden diferir por lecturas no atómicas
		t.Logf("aviso: estados suman más que el total (lectura no atómica): %+v", f)
	}
}
