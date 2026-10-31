package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
)

func TestPrimerErrorCancelaAlResto(t *testing.T) {
	g, ctx := errgroup.WithContext(context.Background())
	ini := time.Now()
	g.Go(func() error { return tarea(ctx, 1, 10*time.Millisecond, true) })
	g.Go(func() error { return tarea(ctx, 2, 5*time.Second, false) })
	err := g.Wait()
	if err == nil || !strings.Contains(err.Error(), "tarea 1 falló") {
		t.Fatalf("err = %v", err)
	}
	if time.Since(ini) > time.Second {
		t.Fatal("la tarea larga no se canceló")
	}
}

func TestCancelacionExternaLlegaATodas(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	g, gctx := errgroup.WithContext(ctx)
	for i := range 5 {
		g.Go(func() error { return tarea(gctx, i, time.Hour, false) })
	}
	cancel()
	if err := g.Wait(); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want Canceled", err)
	}
}

func TestSetLimit(t *testing.T) {
	var g errgroup.Group
	g.SetLimit(2)
	ok := make(chan struct{})
	g.Go(func() error { <-ok; return nil })
	g.Go(func() error { <-ok; return nil })
	if g.TryGo(func() error { return nil }) {
		t.Fatal("TryGo debería fallar con el límite lleno")
	}
	close(ok)
	g.Wait()
}
