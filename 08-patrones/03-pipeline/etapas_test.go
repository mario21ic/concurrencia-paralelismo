package main

import (
	"context"
	"runtime"
	"slices"
	"testing"
	"time"
)

func TestPipeline(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var got []int
	for v := range Tomar(ctx, Mapear(ctx, Filtrar(ctx, Contar(ctx), pares), cuadrado), 4) {
		got = append(got, v)
	}
	if want := []int{4, 16, 36, 64}; !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestCancelarLiberaGoroutines(t *testing.T) {
	base := runtime.NumGoroutine()
	for range 20 {
		ctx, cancel := context.WithCancel(context.Background())
		<-Mapear(ctx, Filtrar(ctx, Contar(ctx), pares), cuadrado)
		cancel()
	}
	time.Sleep(100 * time.Millisecond)
	if d := runtime.NumGoroutine(); d > base+1 {
		t.Fatalf("goroutines base=%d después=%d", base, d)
	}
}

func TestParsearConErrores(t *testing.T) {
	ctx := context.Background()
	var vals, errs int
	for r := range Parsear(ctx, Desde(ctx, "1", "x", "3")) {
		if r.Err != nil {
			errs++
		} else {
			vals++
		}
	}
	if vals != 2 || errs != 1 {
		t.Fatalf("vals=%d errs=%d", vals, errs)
	}
}
