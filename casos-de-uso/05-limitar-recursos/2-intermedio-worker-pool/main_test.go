package main

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func TestIntentarRechazaCuandoNoHaySitio(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := NuevoPool(2, 3)
		bloqueo := make(chan struct{})
		espera := func() { <-bloqueo }
		aceptadas := 0
		for range 10 {
			if p.Intentar(espera) {
				aceptadas++
			}
			synctest.Wait() // deja que los workers cojan tareas antes del siguiente envío
		}
		if aceptadas != 5 { // 2 en los workers + 3 en la cola
			t.Fatalf("aceptadas = %d, esperado 5", aceptadas)
		}
		close(bloqueo)
		p.Cerrar()
		if p.Hechas.Load() != 5 {
			t.Fatalf("hechas = %d", p.Hechas.Load())
		}
	})
}

func TestEnviarFrenaAlProductorSinPerderTareas(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := NuevoPool(2, 2)
		t0 := time.Now()
		for range 12 {
			if err := p.Enviar(t.Context(), func() { time.Sleep(10 * time.Millisecond) }); err != nil {
				t.Fatal(err)
			}
		}
		if time.Since(t0) == 0 {
			t.Fatal("el productor debía haberse frenado en algún momento")
		}
		p.Cerrar()
		if p.Hechas.Load() != 12 {
			t.Fatalf("hechas = %d, esperado 12", p.Hechas.Load())
		}
	})
}

func TestEnviarSeCancelaConElContexto(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := NuevoPool(1, 0)
		var seguir atomic.Bool
		p.Enviar(t.Context(), func() {
			for !seguir.Load() {
				time.Sleep(time.Millisecond)
			}
		})
		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()
		if err := p.Enviar(ctx, func() {}); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err = %v", err)
		}
		seguir.Store(true)
		p.Cerrar()
	})
}
