package main

import (
	"slices"
	"testing"
)

func TestPlaneadorSeDesplaza(t *testing.T) {
	// Un planeador recorre (+1,+1) cada 4 generaciones en una malla toroidal.
	const n = 16
	t0 := Nuevo(n)
	for _, p := range [][2]int{{1, 0}, {2, 1}, {0, 2}, {1, 2}, {2, 2}} { // x, y
		t0.celda[p[1]*n+p[0]] = 1
	}
	got := Evoluciona(t0, 4, 4)
	want := Nuevo(n)
	for _, p := range [][2]int{{2, 1}, {3, 2}, {1, 3}, {2, 3}, {3, 3}} {
		want.celda[p[1]*n+p[0]] = 1
	}
	if !slices.Equal(got.celda, want.celda) {
		t.Fatalf("el planeador no se desplazó como se esperaba")
	}
	if got.Vivas() != 5 {
		t.Fatalf("vivas = %d, esperado 5", got.Vivas())
	}
}

func TestParaleloIgualQueSecuencial(t *testing.T) {
	for _, n := range []int{5, 16, 33, 64} {
		ini := Nuevo(n)
		ini.Aleatorio(uint32(n))
		want := Evoluciona(ini, 25, 1)
		for _, k := range []int{2, 3, 4, 8, 100} {
			if got := Evoluciona(ini, 25, k); !slices.Equal(got.celda, want.celda) {
				t.Errorf("n=%d k=%d: distinto del secuencial", n, k)
			}
		}
	}
}

func TestNoModificaElInicial(t *testing.T) {
	ini := Nuevo(16)
	ini.Aleatorio(3)
	copia := slices.Clone(ini.celda)
	Evoluciona(ini, 10, 4)
	if !slices.Equal(ini.celda, copia) {
		t.Fatal("Evoluciona no debe modificar el tablero inicial")
	}
}
