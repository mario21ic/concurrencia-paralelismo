package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"golang.org/x/sync/errgroup"
)

func TestCapturar(t *testing.T) {
	err := Capturar(func() error { panic("x") })
	pe, ok := EsPanic(err)
	if !ok || pe.Valor != "x" || len(pe.Stack) == 0 {
		t.Fatalf("err=%v pe=%v", err, pe)
	}
}

func TestCapturarConservaCadena(t *testing.T) {
	err := Capturar(func() error { panic(errBase) })
	if !errors.Is(err, errBase) {
		t.Fatal("errors.Is debe ver el error original")
	}
}

func TestSinPanicDevuelveElError(t *testing.T) {
	want := errors.New("normal")
	if got := Capturar(func() error { return want }); got != want {
		t.Fatalf("got %v", got)
	}
}

func TestGoSeguroCancelaAlResto(t *testing.T) {
	g, ctx := errgroup.WithContext(context.Background())
	GoSeguro(g, func() error { panic("boom") })
	GoSeguro(g, func() error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(5 * time.Second):
			return nil
		}
	})
	ini := time.Now()
	err := g.Wait()
	if _, ok := EsPanic(err); !ok {
		t.Fatalf("err=%v, se esperaba un PanicError", err)
	}
	if time.Since(ini) > time.Second {
		t.Fatal("el pánico no canceló al resto")
	}
}
