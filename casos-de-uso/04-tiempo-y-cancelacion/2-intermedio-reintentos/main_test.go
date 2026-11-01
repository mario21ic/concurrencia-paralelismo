package main

import (
	"context"
	"errors"
	"testing"
	"testing/synctest"
	"time"
)

func exacta(base, tope time.Duration, n int) Politica {
	return Politica{MaxIntentos: n, Base: base, Tope: tope, Jitter: func(d time.Duration) time.Duration { return d }}
}

func TestBackoffExponencialExactoConTope(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var instantes []time.Duration
		t0 := time.Now()
		err := Reintentar(t.Context(), exacta(time.Second, 3*time.Second, 5), func(context.Context) error {
			instantes = append(instantes, time.Since(t0))
			return ErrTemporal
		})
		if !errors.Is(err, ErrTemporal) {
			t.Fatalf("err = %v", err)
		}
		// esperas: 1 s, 2 s, 3 s (tope), 3 s
		esperados := []time.Duration{0, time.Second, 3 * time.Second, 6 * time.Second, 9 * time.Second}
		for i, e := range esperados {
			if instantes[i] != e {
				t.Fatalf("intento %d a %v, esperado %v (todos: %v)", i+1, instantes[i], e, instantes)
			}
		}
	})
}

func TestUnErrorPermanenteNoSeReintenta(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := 0
		err := Reintentar(t.Context(), exacta(time.Second, time.Minute, 5), func(context.Context) error { n++; return errors.New("400") })
		if n != 1 || err == nil {
			t.Fatalf("n=%d err=%v", n, err)
		}
	})
}

func TestTerminaEnCuantoTieneExito(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		n := 0
		t0 := time.Now()
		err := Reintentar(t.Context(), exacta(time.Second, time.Minute, 5), func(context.Context) error {
			if n++; n < 3 {
				return ErrTemporal
			}
			return nil
		})
		if err != nil || n != 3 || time.Since(t0) != 3*time.Second { // 1 s + 2 s
			t.Fatalf("err=%v n=%d tras %v", err, n, time.Since(t0))
		}
	})
}

func TestLaEsperaSeCancelaConElContexto(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 1500*time.Millisecond)
		defer cancel()
		t0 := time.Now()
		err := Reintentar(ctx, exacta(time.Second, time.Minute, 10), func(context.Context) error { return ErrTemporal })
		if !errors.Is(err, context.DeadlineExceeded) || time.Since(t0) != 1500*time.Millisecond {
			t.Fatalf("err=%v tras %v: debía cancelarse A MITAD de la espera de 2 s", err, time.Since(t0))
		}
	})
}

func TestJitterPorDefectoQuedaEntreLaMitadYElTotal(t *testing.T) {
	p := Politica{MaxIntentos: 5, Base: time.Second, Tope: time.Minute}
	for range 1000 {
		if d := p.espera(3); d < 2*time.Second || d > 4*time.Second { // 4 s nominal
			t.Fatalf("espera fuera de [2s,4s]: %v", d)
		}
	}
}
