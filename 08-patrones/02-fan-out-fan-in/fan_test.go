package main

import (
	"context"
	"runtime"
	"slices"
	"testing"
	"time"
)

func TestMergeEntregaTodo(t *testing.T) {
	ctx := context.Background()
	m := Merge(ctx, Generar(ctx, 1, 2, 3), Generar(ctx, 10, 20), Generar[int](ctx))
	var got []int
	for v := range m {
		got = append(got, v)
	}
	slices.Sort(got)
	if !slices.Equal(got, []int{1, 2, 3, 10, 20}) {
		t.Fatalf("got %v", got)
	}
}

func TestFanOutCadaValorUnaVez(t *testing.T) {
	ctx := context.Background()
	vals := make([]int, 500)
	for i := range vals {
		vals[i] = i
	}
	var got []int
	for v := range Merge(ctx, FanOut(ctx, Generar(ctx, vals...), 7, func(n int) int { return n })...) {
		got = append(got, v)
	}
	slices.Sort(got)
	if !slices.Equal(got, vals) {
		t.Fatalf("se perdieron o duplicaron valores (len=%d)", len(got))
	}
}

func TestCancelacionSinFugas(t *testing.T) {
	antes := runtime.NumGoroutine()
	ctx, cancel := context.WithCancel(context.Background())
	vals := make([]int, 100000)
	out := Merge(ctx, FanOut(ctx, Generar(ctx, vals...), 4, func(n int) int { return n })...)
	<-out // consumir uno y abandonar
	cancel()
	time.Sleep(100 * time.Millisecond)
	if d := runtime.NumGoroutine(); d > antes+1 {
		t.Fatalf("goroutines antes=%d después=%d", antes, d)
	}
}
