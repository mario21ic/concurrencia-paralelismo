package main

import (
	"iter"
	"runtime"
	"slices"
	"testing"
	"time"
)

func TestMezclar(t *testing.T) {
	got := slices.Collect(Mezclar(slices.Values([]int{1, 4, 7}), slices.Values([]int{2, 3, 8, 9}), menor))
	if want := []int{1, 2, 3, 4, 7, 8, 9}; !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestMezclarConVacia(t *testing.T) {
	got := slices.Collect(Mezclar(slices.Values([]int{}), slices.Values([]int{1, 2}), menor))
	if !slices.Equal(got, []int{1, 2}) {
		t.Fatalf("got %v", got)
	}
}

func TestZipTerminaConLaMasCorta(t *testing.T) {
	n := 0
	for range Zip(slices.Values([]int{1, 2, 3, 4}), slices.Values([]int{1, 2})) {
		n++
	}
	if n != 2 {
		t.Fatalf("pares=%d, want 2", n)
	}
}

func TestBreakLiberaPulls(t *testing.T) {
	base := runtime.NumGoroutine()
	for range 50 {
		for range Mezclar(slices.Values([]int{1, 3, 5}), slices.Values([]int{2, 4, 6}), menor) {
			break // Mezclar debe hacer stop() de sus dos Pull por el defer
		}
	}
	time.Sleep(20 * time.Millisecond)
	if d := runtime.NumGoroutine(); d > base+1 {
		t.Fatalf("fuga: base=%d después=%d", base, d)
	}
}

func TestPullPropagaPanic(t *testing.T) {
	next, stop := iter.Pull(func(yield func(int) bool) { panic("boom") })
	defer stop()
	defer func() {
		if r := recover(); r != "boom" {
			t.Fatalf("recover = %v, want boom", r)
		}
	}()
	next()
}
