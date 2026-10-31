package reloj

import (
	"context"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

// 1) El reloj de la burbuja es virtual: 24 horas pasan al instante.
func TestRelojVirtual(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		inicio := time.Now()
		if want := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC); !inicio.Equal(want) {
			t.Fatalf("la hora inicial de la burbuja es %v, want %v", inicio, want)
		}
		time.Sleep(24 * time.Hour) // retorna de inmediato en tiempo real
		if got := time.Since(inicio); got != 24*time.Hour {
			t.Fatalf("transcurrido = %v, want exactamente 24h", got)
		}
	})
}

// 2) El tiempo avanza SOLO cuando todas las goroutines están bloqueadas, y salta
// hasta el siguiente evento: el orden de despertar es exacto y reproducible.
func TestOrdenDeDespertar(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var mu sync.Mutex
		var orden []int
		var wg sync.WaitGroup
		for _, seg := range []int{3, 1, 2} {
			wg.Go(func() {
				time.Sleep(time.Duration(seg) * time.Second)
				mu.Lock()
				orden = append(orden, seg)
				mu.Unlock()
			})
		}
		wg.Wait()
		if !slices.Equal(orden, []int{1, 2, 3}) {
			t.Fatalf("orden = %v", orden)
		}
	})
}

// 3) Wait: esperar a que el resto de goroutines estén bloqueadas, sin dormir.
func TestWait(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		hecho := false
		go func() { hecho = true }()
		synctest.Wait() // todas las demás goroutines terminaron o están bloqueadas
		if !hecho {
			t.Fatal("la goroutine debería haber corrido")
		}
	})
}

// 4) Wait permite afirmar sobre estados INTERMEDIOS ("todavía no ha pasado").
func TestEstadoIntermedio(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ch := make(chan struct{})
		llego := false
		go func() {
			<-ch
			llego = true
		}()

		synctest.Wait() // la goroutine está bloqueada en <-ch
		if llego {
			t.Fatal("no debería haber llegado todavía")
		}
		close(ch)
		synctest.Wait()
		if !llego {
			t.Fatal("debería haber llegado tras close")
		}
	})
}

// 5) Timers y tickers también usan el reloj virtual.
func TestTicker(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		tk := time.NewTicker(time.Second)
		defer tk.Stop()
		ticks := 0
		fin := time.After(5*time.Second + 500*time.Millisecond)
		for {
			select {
			case <-tk.C:
				ticks++
			case <-fin:
				if ticks != 5 {
					t.Fatalf("ticks = %d, want 5", ticks)
				}
				return
			}
		}
	})
}

// 6) context.WithTimeout vence exactamente en el instante virtual esperado.
func TestContextConPlazo(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
		defer cancel()
		ini := time.Now()
		<-ctx.Done()
		if got := time.Since(ini); got != time.Minute {
			t.Fatalf("venció a los %v, want 1m", got)
		}
		if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Fatalf("Err = %v", ctx.Err())
		}
	})
}

// 7) AfterFunc (de time y de context).
func TestAfterFunc(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// atomic.Bool y no bool: el test LEE la variable antes de que el callback la
		// ESCRIBA y nada establece un orden entre ambos accesos. synctest hace el tiempo
		// determinista, pero NO sustituye la sincronización de memoria: -race lo detecta.
		var llamado atomic.Bool
		time.AfterFunc(10*time.Minute, func() { llamado.Store(true) })

		time.Sleep(10*time.Minute - time.Nanosecond)
		synctest.Wait()
		if llamado.Load() {
			t.Fatal("no debería haberse llamado un nanosegundo antes")
		}
		time.Sleep(time.Nanosecond)
		synctest.Wait()
		if !llamado.Load() {
			t.Fatal("debería haberse llamado a los 10 minutos exactos")
		}

		// context.AfterFunc: se ejecuta tras cancelar.
		ctx, cancel := context.WithCancel(t.Context())
		limpiado := false
		context.AfterFunc(ctx, func() { limpiado = true })
		synctest.Wait()
		if limpiado {
			t.Fatal("antes de cancelar no debe ejecutarse")
		}
		cancel()
		synctest.Wait()
		if !limpiado {
			t.Fatal("tras cancelar debe ejecutarse")
		}
	})
}

// 8) select con timeout: el límite se alcanza sin espera real.
func TestSelectConTimeout(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ch := make(chan int) // nadie enviará
		select {
		case <-ch:
			t.Fatal("no debería recibir")
		case <-time.After(time.Hour):
		}
	})
}

// 9) sync.WaitGroup.Wait y sync.Cond.Wait son bloqueos "durables": el reloj avanza.
func TestWaitGroupYCond(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var wg sync.WaitGroup
		wg.Go(func() { time.Sleep(time.Hour) })
		wg.Wait() // bloquea hasta que pase la hora virtual

		var mu sync.Mutex
		cond := sync.NewCond(&mu)
		listo := false
		go func() {
			time.Sleep(time.Minute)
			mu.Lock()
			listo = true
			cond.Broadcast()
			mu.Unlock()
		}()
		mu.Lock()
		for !listo {
			cond.Wait()
		}
		mu.Unlock()
	})
}
