package main

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func lentoSalvoUno(malo string) Validador {
	return func(ctx context.Context, it string) error {
		if it == malo {
			time.Sleep(10 * time.Millisecond)
			return errInvalido
		}
		select {
		case <-time.After(time.Second):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func TestElPrimerErrorCancelaAlRestoYSeDevuelveEse(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		items := []string{"a", "b", "c", "MALO", "e", "f"}
		t0 := time.Now()
		err := ValidarTodos(t.Context(), items, 4, lentoSalvoUno("MALO"))
		if !errors.Is(err, errInvalido) {
			t.Fatalf("err = %v, esperado errInvalido", err)
		}
		if d := time.Since(t0); d != 10*time.Millisecond {
			t.Fatalf("tardó %v: debía volver en cuanto falló el inválido (10 ms)", d)
		}
	})
}

func TestRespetaElLimiteDeConcurrencia(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var activas, maximo atomic.Int32
		validar := func(ctx context.Context, _ string) error {
			n := activas.Add(1)
			defer activas.Add(-1)
			for {
				m := maximo.Load()
				if n <= m || maximo.CompareAndSwap(m, n) {
					break
				}
			}
			time.Sleep(100 * time.Millisecond)
			return nil
		}
		items := make([]string, 10)
		for i := range items {
			items[i] = fmt.Sprint(i)
		}
		if err := ValidarTodos(t.Context(), items, 3, validar); err != nil || maximo.Load() != 3 {
			t.Fatalf("err=%v máximo=%d (esperado 3)", err, maximo.Load())
		}
	})
}

func TestSinErroresDevuelveNil(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		if err := ValidarTodos(t.Context(), []string{"a", "b"}, 2, lentoSalvoUno("nadie")); err != nil {
			t.Fatal(err)
		}
	})
}
