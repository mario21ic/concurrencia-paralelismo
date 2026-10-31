package main

import (
	"iter"
	"runtime"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

func TestPrefetchOrdenYContenido(t *testing.T) {
	var p atomic.Int32
	got := slices.Collect(Prefetch(fuente(50, 0, &p), 8))
	want := make([]int, 50)
	for i := range want {
		want[i] = i
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestPrefetchBreakSinFugas(t *testing.T) {
	base := runtime.NumGoroutine()
	for range 30 {
		var p atomic.Int32
		for v := range Prefetch(fuente(1_000_000, 0, &p), 4) {
			if v == 2 {
				break
			}
		}
	}
	time.Sleep(30 * time.Millisecond)
	if d := runtime.NumGoroutine(); d > base+1 {
		t.Fatalf("fuga: base=%d después=%d", base, d)
	}
}

func TestPrefetchPropagaPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != "boom" {
			t.Fatalf("recover=%v", r)
		}
	}()
	src := func(yield func(int) bool) {
		yield(1)
		panic("boom")
	}
	for range Prefetch(iter.Seq[int](src), 1) {
	}
}

func TestParMapConservaOrden(t *testing.T) {
	entradas := make([]int, 200)
	for i := range entradas {
		entradas[i] = i
	}
	got := slices.Collect(ParMap(slices.Values(entradas), 8, func(n int) int {
		time.Sleep(time.Duration((200-n)%5) * time.Millisecond) // tiempos desiguales
		return n * 2
	}))
	for i, v := range got {
		if v != i*2 {
			t.Fatalf("got[%d]=%d, want %d", i, v, i*2)
		}
	}
	if len(got) != 200 {
		t.Fatalf("len=%d", len(got))
	}
}

func TestParMapLimiteDeConcurrencia(t *testing.T) {
	var actuales, maximo atomic.Int32
	for range ParMap(slices.Values(make([]int, 100)), 4, func(int) int {
		a := actuales.Add(1)
		for m := maximo.Load(); a > m && !maximo.CompareAndSwap(m, a); m = maximo.Load() {
		}
		time.Sleep(time.Millisecond)
		actuales.Add(-1)
		return 0
	}) {
	}
	if maximo.Load() > 4 {
		t.Fatalf("concurrencia máxima %d, esperada <= 4", maximo.Load())
	}
}

func TestParMapBreakSinFugasYPerezoso(t *testing.T) {
	base := runtime.NumGoroutine()
	var p atomic.Int32
	for v := range ParMap(fuente(1_000_000, 0, &p), 4, func(n int) int { return n }) {
		if v == 2 {
			break
		}
	}
	time.Sleep(30 * time.Millisecond)
	if d := runtime.NumGoroutine(); d > base+1 {
		t.Fatalf("fuga: base=%d después=%d", base, d)
	}
	if p.Load() > 100 {
		t.Fatalf("la fuente produjo %d elementos; debería ser perezosa", p.Load())
	}
}

func TestParMapPropagaPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != "boom" {
			t.Fatalf("recover=%v", r)
		}
	}()
	for range ParMap(slices.Values([]int{1, 2, 3, 4}), 2, func(n int) int {
		if n == 3 {
			panic("boom")
		}
		return n
	}) {
	}
}
