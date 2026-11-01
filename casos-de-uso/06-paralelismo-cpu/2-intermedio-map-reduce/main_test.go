package main

import (
	"maps"
	"testing"
)

func TestLasTresVersionesCoinciden(t *testing.T) {
	palabras := corpus(50_000, 300)
	want := Secuencial(palabras)
	if got := MutexGlobal(palabras, 4); !maps.Equal(got, want) {
		t.Error("MutexGlobal difiere")
	}
	for _, k := range []int{1, 3, 8} {
		if got := LocalYFusion(palabras, k); !maps.Equal(got, want) {
			t.Errorf("LocalYFusion(k=%d) difiere", k)
		}
	}
	var total int
	for _, n := range want {
		total += n
	}
	if total != len(palabras) {
		t.Errorf("total %d != %d", total, len(palabras))
	}
}
