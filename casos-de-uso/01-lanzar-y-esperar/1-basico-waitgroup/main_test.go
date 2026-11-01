package main

import (
	"slices"
	"testing"
	"testing/synctest"
	"time"
)

func TestParaleloTardaLoQueLaEsperaMasLarga(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		urls := []string{"a", "b", "c", "d", "e", "f", "g", "h"}

		t0 := time.Now()
		seq := Secuencial(urls)
		if d := time.Since(t0); d != 8*latencia {
			t.Fatalf("secuencial tardó %v, esperado %v", d, 8*latencia)
		}

		t0 = time.Now()
		par := Paralelo(urls)
		if d := time.Since(t0); d != latencia {
			t.Fatalf("paralelo tardó %v, esperado %v", d, latencia)
		}
		if !slices.Equal(seq, par) {
			t.Fatal("el resultado paralelo debe conservar el orden de las URLs")
		}
	})
}
