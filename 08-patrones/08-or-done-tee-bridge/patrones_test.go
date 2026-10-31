package main

import (
	"context"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"
)

func TestOrDoneCancela(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	in := make(chan int) // nunca emite ni se cierra
	out := OrDone(ctx, in)
	cancel()
	select {
	case _, ok := <-out:
		if ok {
			t.Fatal("no esperaba valores")
		}
	case <-time.After(time.Second):
		t.Fatal("OrDone no respetó la cancelación")
	}
}

func TestTeeAmbosRecibenTodo(t *testing.T) {
	ctx := context.Background()
	a, b := Tee(ctx, generar(ctx, 1, 2, 3, 4, 5))
	var ra, rb []int
	var wg sync.WaitGroup
	wg.Go(func() {
		for v := range a {
			ra = append(ra, v)
		}
	})
	wg.Go(func() {
		for v := range b {
			rb = append(rb, v)
		}
	})
	wg.Wait()
	want := []int{1, 2, 3, 4, 5}
	if !slices.Equal(ra, want) || !slices.Equal(rb, want) {
		t.Fatalf("a=%v b=%v", ra, rb)
	}
}

func TestBridgeEnOrden(t *testing.T) {
	ctx := context.Background()
	flujo := make(chan (<-chan int))
	go func() {
		defer close(flujo)
		flujo <- generar(ctx, 1, 2)
		flujo <- generar(ctx, 3)
		flujo <- generar(ctx)
		flujo <- generar(ctx, 4, 5)
	}()
	var got []int
	for v := range Bridge(ctx, flujo) {
		got = append(got, v)
	}
	if !slices.Equal(got, []int{1, 2, 3, 4, 5}) {
		t.Fatalf("got %v", got)
	}
}

func TestSinFugasTrasCancelar(t *testing.T) {
	base := runtime.NumGoroutine()
	ctx, cancel := context.WithCancel(context.Background())
	a, b := Tee(ctx, generar(ctx, make([]int, 1000)...))
	<-a
	_ = b // b no se lee: Tee queda bloqueado hasta cancelar
	cancel()
	time.Sleep(100 * time.Millisecond)
	if d := runtime.NumGoroutine(); d > base+1 {
		t.Fatalf("goroutines base=%d después=%d", base, d)
	}
}
