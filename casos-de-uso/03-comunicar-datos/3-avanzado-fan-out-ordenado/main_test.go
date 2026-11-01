package main

import (
	"context"
	"slices"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

// fuente emite 0..n-1. Quien crea la fuente es quien debe poder detenerla: Procesar no es dueño de `in`.
func fuente(ctx context.Context, n int) <-chan int {
	ch := make(chan int)
	go func() {
		defer close(ch)
		for i := range n {
			select {
			case ch <- i:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}

func TestConservaElOrdenAunqueLosWorkersTerminenDesordenados(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		retardo := func(v int) int {
			time.Sleep(time.Duration((7-v%7)*10) * time.Millisecond) // los primeros tardan más
			return v * 10
		}
		var got []int
		for v := range Procesar(t.Context(), fuente(t.Context(), 50), 4, retardo) {
			got = append(got, v)
		}
		esperado := make([]int, 50)
		for i := range esperado {
			esperado[i] = i * 10
		}
		if !slices.Equal(got, esperado) {
			t.Fatalf("orden roto: %v", got)
		}
	})
}

func TestEsMasRapidoQueSecuencial(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := func(v int) int { time.Sleep(10 * time.Millisecond); return v }
		t0 := time.Now()
		n := 0
		for range Procesar(t.Context(), fuente(t.Context(), 40), 4, f) {
			n++
		}
		if d := time.Since(t0); n != 40 || d != 100*time.Millisecond { // 40 elementos / 4 workers × 10 ms
			t.Fatalf("n=%d tardó %v, esperado 100 ms", n, d)
		}
	})
}

func TestLaVentanaAcotaLosElementosEnVuelo(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var enVuelo, maximo atomic.Int32
		bloqueo := make(chan struct{})
		f := func(v int) int {
			n := enVuelo.Add(1)
			for {
				m := maximo.Load()
				if n <= m || maximo.CompareAndSwap(m, n) {
					break
				}
			}
			if v == 0 {
				<-bloqueo // el elemento 0 es muy lento: los demás se acumulan esperando el orden
			}
			enVuelo.Add(-1)
			return v
		}
		out := Procesar(t.Context(), fuente(t.Context(), 1000), 4, f)
		synctest.Wait()
		// No se leyó nada: como mucho 2*workers elementos pueden haber entrado al sistema.
		if maximo.Load() > 4 {
			t.Fatalf("más de 4 workers a la vez: %d", maximo.Load())
		}
		close(bloqueo)
		n := 0
		for range out {
			n++
		}
		if n != 1000 {
			t.Fatalf("n = %d", n)
		}
	})
}

func TestCancelarLiberaTodo(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	out := Procesar(ctx, fuente(ctx, 1_000_000), 4, func(v int) int { return v })
	for range 10 {
		<-out
	}
	cancel()
	for range out {
	}
}
