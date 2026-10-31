package main

import (
	"slices"
	"testing"
)

func TestComposicion(t *testing.T) {
	got := slices.Collect(Take(Map(Filter(Contar(1), par), func(n int) int { return n * n }), 4))
	if want := []int{4, 16, 36, 64}; !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestPerezosidad(t *testing.T) {
	llamadas := 0
	src := Map(Contar(1), func(n int) int { llamadas++; return n })
	for range Take(src, 3) {
	}
	if llamadas != 3 {
		t.Fatalf("se calcularon %d elementos; deberían ser 3 (perezoso)", llamadas)
	}
}

func TestBreakDetieneIterador(t *testing.T) {
	limpio := false
	s := func(yield func(int) bool) {
		defer func() { limpio = true }()
		for i := 0; ; i++ {
			if !yield(i) {
				return
			}
		}
	}
	for v := range s {
		if v == 3 {
			break
		}
	}
	if !limpio {
		t.Fatal("el defer del iterador no se ejecutó tras el break")
	}
}

func TestIteradorMaloPanica(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("se esperaba panic")
		}
	}()
	for v := range malo() {
		if v == 1 {
			break
		}
	}
}
