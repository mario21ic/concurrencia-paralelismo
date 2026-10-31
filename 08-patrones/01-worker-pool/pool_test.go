package main

import (
	"context"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func TestOrdenYLimite(t *testing.T) {
	entradas := make([]int, 100)
	for i := range entradas {
		entradas[i] = i
	}
	var actuales, maximo atomic.Int32
	res := Procesar(context.Background(), 5, entradas, func(_ context.Context, n int) (int, error) {
		a := actuales.Add(1)
		for m := maximo.Load(); a > m && !maximo.CompareAndSwap(m, a); m = maximo.Load() {
		}
		time.Sleep(time.Millisecond)
		actuales.Add(-1)
		return n * 2, nil
	})
	for i, r := range res {
		if r.Err != nil || r.Valor != i*2 {
			t.Fatalf("res[%d] = %+v", i, r)
		}
	}
	if maximo.Load() > 5 {
		t.Fatalf("concurrencia máxima %d > 5 workers", maximo.Load())
	}
}

func TestCancelacion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var hechos atomic.Int32
	entradas := make([]int, 1000)
	res := Procesar(ctx, 4, entradas, func(_ context.Context, _ int) (int, error) {
		if hechos.Add(1) == 10 {
			cancel()
		}
		time.Sleep(time.Millisecond)
		return 0, nil
	})
	cancelados := 0
	for _, r := range res {
		if r.Err != nil {
			cancelados++
		}
	}
	if cancelados < 900 {
		t.Fatalf("solo %d cancelados; se esperaban casi todos", cancelados)
	}
}

func TestSinFugas(t *testing.T) {
	antes := runtime.NumGoroutine()
	for range 20 {
		Procesar(context.Background(), 8, make([]int, 50), func(_ context.Context, n int) (int, error) { return n, nil })
	}
	time.Sleep(20 * time.Millisecond)
	if d := runtime.NumGoroutine(); d > antes+1 {
		t.Fatalf("goroutines antes=%d después=%d", antes, d)
	}
}

func TestEntradaVacia(t *testing.T) {
	if res := Procesar(context.Background(), 4, []int{}, func(_ context.Context, n int) (int, error) { return n, nil }); len(res) != 0 {
		t.Fatal("se esperaba resultado vacío")
	}
}
