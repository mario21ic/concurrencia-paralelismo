package main

import (
	"math"
	"testing"
)

func TestAmdahlFormula(t *testing.T) {
	casos := []struct {
		s    float64
		k    int
		want float64
	}{
		{0, 8, 8},
		{1, 8, 1},
		{0.5, 2, 4.0 / 3},
		{0.25, 4, 2.2857142857},
	}
	for _, c := range casos {
		if got := Amdahl(c.s, c.k); math.Abs(got-c.want) > 1e-6 {
			t.Errorf("Amdahl(%v,%d)=%v, esperado %v", c.s, c.k, got, c.want)
		}
	}
	if Amdahl(0.1, 1_000_000) > 10 {
		t.Error("el techo con s=10 % es 10×")
	}
}

func TestTareaNoDependeDeK(t *testing.T) {
	want := Tarea(640_000, 0.25, 1)
	for _, k := range []int{2, 3, 8} {
		if got := Tarea(640_000, 0.25, k); got != want {
			t.Errorf("k=%d: %x, esperado %x", k, got, want)
		}
	}
}
