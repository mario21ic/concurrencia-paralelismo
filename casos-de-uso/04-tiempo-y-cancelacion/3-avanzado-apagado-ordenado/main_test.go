package main

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

func dormir(d time.Duration) Tarea {
	return func(ctx context.Context) {
		select {
		case <-time.After(d):
		case <-ctx.Done():
		}
	}
}

func TestCerrarDrenaLoPendiente(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := Iniciar(2, 10)
		for range 6 {
			if err := s.Enviar(dormir(100 * time.Millisecond)); err != nil {
				t.Fatal(err)
			}
		}
		t0 := time.Now()
		if !s.Cerrar(time.Second) || s.Hechas.Load() != 6 || time.Since(t0) != 300*time.Millisecond { // 6 tareas / 2 workers × 100 ms
			t.Fatalf("hechas=%d tardó %v", s.Hechas.Load(), time.Since(t0))
		}
	})
}

func TestEnviarTrasCerrarDevuelveErrorSinPanico(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := Iniciar(1, 1)
		s.Cerrar(time.Second)
		if err := s.Enviar(dormir(0)); !errors.Is(err, ErrCerrado) {
			t.Fatalf("err = %v", err)
		}
		s.Cerrar(time.Second) // idempotente: no hace close dos veces
	})
}

func TestColaLlenaRechazaSinBloquear(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := Iniciar(1, 1)
		s.Enviar(dormir(time.Hour)) // la toma el worker
		synctest.Wait()
		s.Enviar(dormir(time.Hour)) // ocupa la única plaza de la cola
		if err := s.Enviar(dormir(0)); !errors.Is(err, ErrLlena) {
			t.Fatalf("err = %v", err)
		}
		s.Cerrar(10 * time.Millisecond)
	})
}

func TestGraciaInsuficienteCancelaLasTareasEnCurso(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := Iniciar(2, 10)
		var canceladas atomic.Int32
		larga := func(ctx context.Context) {
			select {
			case <-time.After(time.Hour):
			case <-ctx.Done():
				canceladas.Add(1)
			}
		}
		for range 4 {
			s.Enviar(larga)
		}
		t0 := time.Now()
		completo := s.Cerrar(200 * time.Millisecond)
		if completo || time.Since(t0) != 200*time.Millisecond || canceladas.Load() != 4 {
			t.Fatalf("completo=%v tardó %v canceladas=%d", completo, time.Since(t0), canceladas.Load())
		}
	})
}

// Enviar y Cerrar concurrentes, con tiempo real y -race: el RWMutex debe evitar "send on closed channel".
func TestEnviarYCerrarConcurrentesNoEntranEnPanico(t *testing.T) {
	for range 50 {
		s := Iniciar(4, 8)
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				for range 200 {
					if s.Enviar(func(context.Context) {}) == ErrCerrado {
						return
					}
				}
			})
		}
		s.Cerrar(time.Second)
		wg.Wait()
	}
}
