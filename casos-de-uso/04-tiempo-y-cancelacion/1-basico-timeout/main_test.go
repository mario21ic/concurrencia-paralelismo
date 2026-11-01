package main

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"
)

func TestSinCtxVuelveAlVencerElPlazoYNoFuga(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		t0 := time.Now()
		_, err := ConPlazoSinCtx(200*time.Millisecond, lenta)
		if !errors.Is(err, ErrPlazo) || time.Since(t0) != 200*time.Millisecond {
			t.Fatalf("err=%v tras %v", err, time.Since(t0))
		}
		time.Sleep(time.Second) // la operación sigue y termina sola; synctest comprobará que no queda nada
	})
}

func TestSinCtxDevuelveElResultadoSiLlegaATiempo(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		v, err := ConPlazoSinCtx(2*time.Second, lenta)
		if v != "respuesta" || err != nil {
			t.Fatalf("%q %v", v, err)
		}
	})
}

func TestCtxCancelaLaOperacion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		t0 := time.Now()
		_, err := ConPlazoCtx(t.Context(), 200*time.Millisecond, lentaCtx)
		if !errors.Is(err, ErrPlazo) || time.Since(t0) != 200*time.Millisecond {
			t.Fatalf("err=%v tras %v", err, time.Since(t0))
		}
	})
}

func TestCtxRespetaLaCancelacionDelPadre(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		time.AfterFunc(50*time.Millisecond, cancel)
		_, err := ConPlazoCtx(ctx, time.Hour, lentaCtx)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, esperado context.Canceled", err)
		}
	})
}
