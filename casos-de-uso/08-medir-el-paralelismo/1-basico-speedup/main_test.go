package main

import (
	"runtime"
	"testing"
)

func TestGastaEsDeterminista(t *testing.T) {
	if Gasta(1000, 7) != Gasta(1000, 7) {
		t.Fatal("Gasta debe ser determinista")
	}
	if Gasta(1000, 7) == Gasta(1000, 8) {
		t.Fatal("semillas distintas deberían dar resultados distintos")
	}
}

func TestEjecutarNoDependeDeK(t *testing.T) {
	want := Ejecutar(40, 2000, 1)
	for _, k := range []int{0, 2, 3, 8, 64} {
		if got := Ejecutar(40, 2000, k); got != want {
			t.Errorf("k=%d: %x, esperado %x", k, got, want)
		}
	}
}

func TestEjecutarConUnSoloNucleo(t *testing.T) {
	// Con GOMAXPROCS=1 hay concurrencia pero el resultado debe seguir siendo el mismo.
	want := Ejecutar(40, 2000, 1)
	prev := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(prev)
	if got := Ejecutar(40, 2000, 8); got != want {
		t.Fatalf("%x, esperado %x", got, want)
	}
}
