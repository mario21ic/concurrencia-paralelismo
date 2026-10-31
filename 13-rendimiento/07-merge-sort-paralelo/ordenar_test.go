package main

import (
	"math/rand/v2"
	"slices"
	"sync/atomic"
	"testing"
)

func casos() map[string][]int {
	r := rand.New(rand.NewPCG(1, 1))
	gen := func(n, rango int) []int {
		a := make([]int, n)
		for i := range a {
			a[i] = r.IntN(rango)
		}
		return a
	}
	asc, desc := make([]int, 5000), make([]int, 5000)
	for i := range asc {
		asc[i], desc[i] = i, 5000-i
	}
	m := map[string][]int{
		"vacío":        {},
		"uno":          {5},
		"dos":          {2, 1},
		"iguales":      slices.Repeat([]int{7}, 3000),
		"ascendente":   asc,
		"descendente":  desc,
		"muchos dups":  gen(50_000, 10),
		"aleatorio 1k": gen(1000, 1<<30),
		"aleatorio 1M": gen(1_000_000, 1<<30),
	}
	for _, n := range []int{3, 24, 25, 100, 4097} {
		m["aleatorio pequeño"+string(rune('a'+n%26))] = gen(n, 1000)
	}
	return m
}

func TestTodasLasVersionesOrdenanBien(t *testing.T) {
	configs := map[string]Config{
		"par u=1":          {Umbral: 1},
		"par u=100":        {Umbral: 100},
		"par u=10000":      {Umbral: 10000},
		"par+mezcla u=1":   {Umbral: 1, MezclaPar: true, UmbralMezcla: 1},
		"par+mezcla u=500": {Umbral: 500, MezclaPar: true, UmbralMezcla: 64},
		"par+mezcla 100k":  {Umbral: 100_000, MezclaPar: true, UmbralMezcla: 10_000},
	}
	for nombre, caso := range casos() {
		esperado := slices.Clone(caso)
		slices.Sort(esperado)

		a := slices.Clone(caso)
		OrdenarSecuencial(a, make([]int, len(a)))
		if !slices.Equal(a, esperado) {
			t.Fatalf("secuencial falla en %q", nombre)
		}
		for cn, c := range configs {
			if len(caso) > 100_000 && c.Umbral < 100 {
				continue // evitar millones de goroutines en el caso grande
			}
			a := slices.Clone(caso)
			OrdenarParalelo(a, make([]int, len(a)), c)
			if !slices.Equal(a, esperado) {
				t.Fatalf("%s falla en %q", cn, nombre)
			}
		}
	}
}

func TestUmbralLimitaLasGoroutines(t *testing.T) {
	a := rand.Perm(1 << 16)
	var pocas, muchas atomic.Int64
	OrdenarParalelo(slices.Clone(a), make([]int, len(a)), Config{Umbral: 1 << 14, Goroutines: &pocas})
	OrdenarParalelo(slices.Clone(a), make([]int, len(a)), Config{Umbral: 64, Goroutines: &muchas})
	if pocas.Load() >= muchas.Load() {
		t.Fatalf("pocas=%d muchas=%d: un umbral mayor debe crear menos goroutines", pocas.Load(), muchas.Load())
	}
	if pocas.Load() != 3 { // 65536 → 2×32768 → 4×16384 (<= umbral): 1 + 2 = 3 divisiones
		t.Fatalf("goroutines = %d, want 3", pocas.Load())
	}
}

func BenchmarkOrdenar(b *testing.B) {
	orig := rand.Perm(1 << 20)
	a := make([]int, len(orig))
	b.Run("slices.Sort", func(b *testing.B) {
		for b.Loop() {
			copy(a, orig)
			slices.Sort(a)
		}
	})
	b.Run("secuencial", func(b *testing.B) {
		tmp := make([]int, len(a))
		for b.Loop() {
			copy(a, orig)
			OrdenarSecuencial(a, tmp)
		}
	})
	b.Run("paralelo", func(b *testing.B) {
		tmp := make([]int, len(a))
		for b.Loop() {
			copy(a, orig)
			OrdenarParalelo(a, tmp, Config{Umbral: 1 << 15})
		}
	})
}
