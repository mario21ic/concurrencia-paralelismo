package main

import (
	"context"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func TestElLimiteNuncaSeSupera(t *testing.T) {
	for _, max := range []int{1, 3, 7} {
		synctest.Test(t, func(t *testing.T) {
			var m Monitor
			t0 := time.Now()
			ConChannel(20, max, func() { m.Entrar(); time.Sleep(100 * time.Millisecond); m.Salir() })
			if int(m.Maximo.Load()) != max {
				t.Errorf("límite %d: máximo observado %d", max, m.Maximo.Load())
			}
			esperado := time.Duration((20+max-1)/max) * 100 * time.Millisecond
			if d := time.Since(t0); d != esperado {
				t.Errorf("límite %d: tardó %v, esperado %v", max, d, esperado)
			}
		})
	}
}

func TestElPresupuestoPonderadoNoSeSupera(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var enUso, pico atomic.Int64
		err := ConPeso(t.Context(), 100, []int64{60, 30, 50, 20, 40, 10, 70}, func(w int64) {
			n := enUso.Add(w)
			for {
				x := pico.Load()
				if n <= x || pico.CompareAndSwap(x, n) {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
			enUso.Add(-w)
		})
		if err != nil || pico.Load() > 100 {
			t.Fatalf("err=%v pico=%d (máx. 100)", err, pico.Load())
		}
	})
}

func TestConPesoSeCancela(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()
		err := ConPeso(ctx, 10, []int64{10, 10, 10}, func(int64) { time.Sleep(time.Second) })
		if err == nil {
			t.Fatal("esperaba error de contexto al agotarse el plazo esperando plaza")
		}
	})
}
