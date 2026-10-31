package main

import (
	"context"
	"slices"
	"testing"
	"time"
)

func TestBatchPorTamano(t *testing.T) {
	in := make(chan int)
	go func() {
		defer close(in)
		for i := 1; i <= 10; i++ {
			in <- i
		}
	}()
	var lotes [][]int
	for l := range Batch(context.Background(), in, 4, time.Hour) {
		lotes = append(lotes, l)
	}
	if len(lotes) != 3 || len(lotes[0]) != 4 || len(lotes[2]) != 2 {
		t.Fatalf("lotes: %v", lotes)
	}
	var todo []int
	for _, l := range lotes {
		todo = append(todo, l...)
	}
	if !slices.Equal(todo, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}) {
		t.Fatalf("se perdieron o reordenaron elementos: %v", todo)
	}
}

func TestBatchPorTiempo(t *testing.T) {
	in := make(chan int)
	out := Batch(context.Background(), in, 100, 30*time.Millisecond)
	in <- 1
	in <- 2
	select {
	case l := <-out:
		if !slices.Equal(l, []int{1, 2}) {
			t.Fatalf("lote %v", l)
		}
	case <-time.After(time.Second):
		t.Fatal("el lote no se emitió por tiempo")
	}
	close(in)
}

func TestBatchCancelacion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	in := make(chan int)
	out := Batch(ctx, in, 10, time.Hour)
	in <- 1
	cancel()
	select {
	case _, ok := <-out:
		if ok {
			t.Fatal("no debería emitir tras cancelar")
		}
	case <-time.After(time.Second):
		t.Fatal("Batch no terminó al cancelar")
	}
}
