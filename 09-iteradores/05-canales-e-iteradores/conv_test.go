package main

import (
	"context"
	"errors"
	"runtime"
	"slices"
	"sync/atomic"
	"testing"
	"time"
)

func TestDeCanal(t *testing.T) {
	ch := make(chan int, 3)
	ch <- 1
	ch <- 2
	ch <- 3
	close(ch)
	if got := slices.Collect(DeCanal(ch)); !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("got %v", got)
	}
}

func TestACanalCancelaSinFugas(t *testing.T) {
	base := runtime.NumGoroutine()
	ctx, cancel := context.WithCancel(context.Background())
	infinito := func(yield func(int) bool) {
		for i := 0; yield(i); i++ {
		}
	}
	ch := ACanal(ctx, infinito)
	<-ch
	cancel()
	time.Sleep(50 * time.Millisecond)
	if d := runtime.NumGoroutine(); d > base+1 {
		t.Fatalf("fuga: base=%d después=%d", base, d)
	}
}

func TestPaginacionPerezosa(t *testing.T) {
	var pedidas atomic.Int32
	n := 0
	for _, err := range Elementos(context.Background(), apiSimulada(&pedidas, -1)) {
		if err != nil {
			t.Fatal(err)
		}
		if n++; n == 4 { // 4 elementos => necesita 2 páginas (3 por página)
			break
		}
	}
	if pedidas.Load() != 2 {
		t.Fatalf("páginas pedidas=%d, want 2", pedidas.Load())
	}
}

func TestPaginacionCompleta(t *testing.T) {
	var pedidas atomic.Int32
	n := 0
	for range Elementos(context.Background(), apiSimulada(&pedidas, -1)) {
		n++
	}
	if n != 30 || pedidas.Load() != 10 {
		t.Fatalf("elementos=%d páginas=%d", n, pedidas.Load())
	}
}

func TestPaginacionError(t *testing.T) {
	var pedidas atomic.Int32
	var ultimo error
	n := 0
	for _, err := range Elementos(context.Background(), apiSimulada(&pedidas, 1)) {
		if err != nil {
			ultimo = err
			break
		}
		n++
	}
	if ultimo == nil || n != 3 {
		t.Fatalf("n=%d err=%v; se esperaban 3 elementos y un error", n, ultimo)
	}
}

func TestPaginacionContextoCancelado(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var pedidas atomic.Int32
	for _, err := range Elementos(ctx, apiSimulada(&pedidas, -1)) {
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err=%v, want Canceled", err)
		}
	}
	if pedidas.Load() != 0 {
		t.Fatal("no debería haber pedido ninguna página")
	}
}
