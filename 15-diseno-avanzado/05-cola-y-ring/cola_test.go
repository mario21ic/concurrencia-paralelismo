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

func TestColaFIFOyCapacidad(t *testing.T) {
	q := NuevaCola[int](3)
	for i := range 3 {
		if !q.TryPut(i) {
			t.Fatalf("TryPut(%d) falló con hueco", i)
		}
	}
	if q.TryPut(99) {
		t.Fatal("TryPut debe fallar si está llena")
	}
	for i := range 3 {
		if v, err := q.Get(t.Context()); err != nil || v != i {
			t.Fatalf("Get = %d,%v; esperado %d", v, err, i)
		}
	}
}

func TestPutBloqueaHastaQueHayaHueco(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		q := NuevaCola[int](1)
		q.TryPut(1)
		hecho := make(chan error)
		go func() { hecho <- q.Put(t.Context(), 2) }()
		synctest.Wait() // Put bloqueado
		select {
		case <-hecho:
			t.Fatal("Put no debería haber terminado")
		default:
		}
		q.Get(t.Context())
		if err := <-hecho; err != nil {
			t.Fatal(err)
		}
		if v, _ := q.Get(t.Context()); v != 2 {
			t.Fatalf("got %d", v)
		}
	})
}

func TestCancelarDespiertaPutYGet(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		llena := NuevaCola[int](1)
		llena.TryPut(1)
		vacia := NuevaCola[int](1)
		ctx, cancel := context.WithTimeout(t.Context(), time.Second)
		defer cancel()
		var wg sync.WaitGroup
		var e1, e2 error
		wg.Go(func() { e1 = llena.Put(ctx, 2) })
		wg.Go(func() { _, e2 = vacia.Get(ctx) })
		wg.Wait() // el reloj virtual avanza 1 s y vence el plazo
		if !errors.Is(e1, context.DeadlineExceeded) || !errors.Is(e2, context.DeadlineExceeded) {
			t.Fatalf("errores: %v, %v", e1, e2)
		}
	})
}

func TestCloseEntregaPendientesYLuegoError(t *testing.T) {
	q := NuevaCola[int](4)
	q.TryPut(1)
	q.TryPut(2)
	q.Close()
	q.Close() // idempotente
	if err := q.Put(t.Context(), 3); !errors.Is(err, ErrCerrada) {
		t.Fatalf("Put tras Close: %v", err)
	}
	for _, esperado := range []int{1, 2} {
		if v, err := q.Get(t.Context()); err != nil || v != esperado {
			t.Fatalf("Get = %d,%v", v, err)
		}
	}
	if _, err := q.Get(t.Context()); !errors.Is(err, ErrCerrada) {
		t.Fatalf("Get final: %v", err)
	}
}

// Cada elemento sale exactamente una vez, con 4 productores y 4 consumidores.
func TestColaMPMC(t *testing.T) {
	q := NuevaCola[int](8)
	const prod, cada = 4, 5000
	var suma, cuenta atomic.Int64
	var wp, wc sync.WaitGroup
	for p := range prod {
		wp.Go(func() {
			for i := range cada {
				q.Put(t.Context(), p*cada+i+1)
			}
		})
	}
	for range 4 {
		wc.Go(func() {
			for {
				v, err := q.Get(t.Context())
				if err != nil {
					return
				}
				suma.Add(int64(v))
				cuenta.Add(1)
			}
		})
	}
	wp.Wait()
	q.Close()
	wc.Wait()
	n := int64(prod * cada)
	if cuenta.Load() != n || suma.Load() != n*(n+1)/2 {
		t.Fatalf("cuenta=%d suma=%d, esperado %d y %d", cuenta.Load(), suma.Load(), n, n*(n+1)/2)
	}
}

func TestRingSPSCOrdenYVuelta(t *testing.T) {
	r := NuevoRingSPSC[int](4)
	for i := range 4 {
		if !r.TryPush(i) {
			t.Fatal("no cabe")
		}
	}
	if r.TryPush(9) {
		t.Fatal("debería estar lleno")
	}
	const total = 200_000
	var wg sync.WaitGroup
	wg.Go(func() {
		for i := 4; i < total; i++ {
			for !r.TryPush(i) {
			}
		}
	})
	for esperado := 0; esperado < total; {
		if v, ok := r.TryPop(); ok {
			if v != esperado {
				t.Fatalf("orden roto: %d, esperado %d", v, esperado)
			}
			esperado++
		}
	}
	wg.Wait()
	if _, ok := r.TryPop(); ok {
		t.Fatal("debería estar vacío")
	}
}

func TestRingMPMC(t *testing.T) {
	r := NuevoRingMPMC[int](64)
	const prod, cada = 4, 20000
	var suma, cuenta atomic.Int64
	var wp, wc sync.WaitGroup
	var hecho atomic.Bool
	for p := range prod {
		wp.Go(func() {
			for i := range cada {
				for !r.TryPush(p*cada + i + 1) {
				}
			}
		})
	}
	for range 4 {
		wc.Go(func() {
			for {
				if v, ok := r.TryPop(); ok {
					suma.Add(int64(v))
					cuenta.Add(1)
				} else if hecho.Load() {
					if v, ok := r.TryPop(); ok { // último vaciado tras ver "hecho"
						suma.Add(int64(v))
						cuenta.Add(1)
						continue
					}
					return
				}
			}
		})
	}
	wp.Wait()
	hecho.Store(true)
	wc.Wait()
	n := int64(prod * cada)
	if cuenta.Load() != n || suma.Load() != n*(n+1)/2 {
		t.Fatalf("cuenta=%d suma=%d, esperado %d y %d", cuenta.Load(), suma.Load(), n, n*(n+1)/2)
	}
}
