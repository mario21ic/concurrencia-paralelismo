package main

import "testing"

func TestLaVersionRapidaEsEquivalente(t *testing.T) {
	var c claveRapida
	for _, i := range []int{0, 1, 42, 999_999, 123456789} {
		if got, want := string(c.calcular(i)), claveLenta(i); got != want {
			t.Fatalf("i=%d: rápida=%s lenta=%s", i, got, want)
		}
	}
	if trabajar("lento", 10_000, 4) != trabajar("rapido", 10_000, 3) {
		t.Fatal("las sumas de comprobación difieren")
	}
}

func BenchmarkClaveLenta(b *testing.B) {
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		claveLenta(i)
	}
}

func BenchmarkClaveRapida(b *testing.B) {
	b.ReportAllocs()
	var c claveRapida
	for i := 0; b.Loop(); i++ {
		c.calcular(i)
	}
}
