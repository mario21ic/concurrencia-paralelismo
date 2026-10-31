package main

import (
	"context"
	"errors"
	"runtime"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

// base deja que terminen las goroutines de tests anteriores antes de contar.
func base() int {
	time.Sleep(50 * time.Millisecond)
	return runtime.NumGoroutine()
}

func TestRelojRunSeParaConCtx(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := NuevoReloj(10 * time.Millisecond)
		ctx, cancel := context.WithCancel(t.Context())
		hecho := make(chan error)
		go func() { hecho <- r.Run(ctx) }()
		time.Sleep(95 * time.Millisecond)
		cancel()
		if err := <-hecho; !errors.Is(err, context.Canceled) {
			t.Fatalf("Run = %v", err)
		}
		if r.Ticks() != 9 {
			t.Fatalf("ticks = %d, esperado 9", r.Ticks())
		}
	})
}

func TestRelojMalFugaUnaGoroutinePorInstancia(t *testing.T) {
	antes := base()
	relojes := make([]*RelojMal, 5)
	for i := range relojes {
		relojes[i] = NuevoRelojMal(time.Hour)
	}
	if d := runtime.NumGoroutine() - antes; d != 5 {
		t.Fatalf("goroutines nuevas = %d, esperado 5 (una por reloj, sin forma pública de pararlas)", d)
	}
	for _, r := range relojes { // limpieza solo para el test
		close(r.parar)
	}
}

func TestDuplicarMalFugaSiElConsumidorAbandona(t *testing.T) {
	antes := base()
	out := DuplicarMal([]int{1, 2, 3, 4})
	<-out // lee uno y se va
	time.Sleep(20 * time.Millisecond)
	if d := runtime.NumGoroutine() - antes; d != 1 {
		t.Fatalf("goroutines nuevas = %d, esperado 1 (bloqueada enviando)", d)
	}
	for range out { // limpieza
	}
}

func TestDuplicarNoFugaSiElConsumidorAbandona(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	out := Duplicar(ctx, []int{1, 2, 3, 4})
	if v := <-out; v != 2 {
		t.Fatalf("primer valor %d", v)
	}
	cancel()
	for range out { // el productor cierra el channel al salir
	}
	// goleak (TestMain) comprobará al final que no queda ninguna goroutine.
}

func TestDuplicarIterNoTieneGoroutinesYAceptaBreak(t *testing.T) {
	antes := base()
	var vistos []int
	for v := range DuplicarIter([]int{1, 2, 3, 4}) {
		vistos = append(vistos, v)
		if v == 4 {
			break
		}
	}
	if len(vistos) != 2 || runtime.NumGoroutine() != antes {
		t.Fatalf("vistos=%v goroutines=%d (antes %d)", vistos, runtime.NumGoroutine(), antes)
	}
}

func TestServicioCloseEsperaYEsIdempotente(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var salio atomic.Bool
		s := Iniciar(t.Context(), func(ctx context.Context) error {
			<-ctx.Done()
			time.Sleep(50 * time.Millisecond) // limpieza que Close debe ESPERAR
			salio.Store(true)
			return ctx.Err()
		})
		if err := s.Close(); err != nil {
			t.Fatalf("Close = %v", err)
		}
		if !salio.Load() {
			t.Fatal("Close volvió antes de que la tarea terminara su limpieza")
		}
		if err := s.Close(); err != nil { // segunda llamada
			t.Fatalf("segundo Close = %v", err)
		}
	})
}

func TestServicioPropagaErrorYPanico(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		boom := errors.New("boom")
		s := Iniciar(t.Context(), func(context.Context) error { return boom })
		<-s.Hecho()
		if err := s.Close(); !errors.Is(err, boom) {
			t.Fatalf("Close = %v", err)
		}
		p := Iniciar(t.Context(), func(context.Context) error { panic("kaboom") })
		<-p.Hecho()
		if err := p.Close(); err == nil || err.Error() != "pánico: kaboom" {
			t.Fatalf("Close = %v", err)
		}
	})
}

// fakeFetch: la URL "mala" falla a los 10 ms; las demás tardan 1 s (respetando la cancelación).
func fakeFetch(ctx context.Context, u string) (string, error) {
	if u == "mala" {
		time.Sleep(10 * time.Millisecond)
		return "", errors.New("falló")
	}
	select {
	case <-time.After(time.Second):
		return "ok:" + u, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func TestDescargarTodasCancelaAlPrimerErrorYEsperaAlResto(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		t0 := time.Now()
		_, err := DescargarTodas(t.Context(), []string{"a", "mala", "b", "c"}, 4, fakeFetch)
		if err == nil || time.Since(t0) != 10*time.Millisecond {
			t.Fatalf("err=%v tras %v; esperado error a los 10 ms exactos (el resto se canceló)", err, time.Since(t0))
		}
		// synctest.Test fallaría aquí si quedara alguna goroutine viva: Wait las esperó a todas.
	})
}

func TestDescargarTodasLimitaLaConcurrencia(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var activas, maxActivas atomic.Int32
		f := func(ctx context.Context, u string) (string, error) {
			n := activas.Add(1)
			defer activas.Add(-1)
			for {
				m := maxActivas.Load()
				if n <= m || maxActivas.CompareAndSwap(m, n) {
					break
				}
			}
			time.Sleep(100 * time.Millisecond)
			return u, nil
		}
		urls := make([]string, 10)
		res, err := DescargarTodas(t.Context(), urls, 3, f)
		if err != nil || len(res) != 10 || maxActivas.Load() != 3 {
			t.Fatalf("err=%v máx. concurrentes=%d (esperado 3)", err, maxActivas.Load())
		}
	})
}

func TestDescargarTodasConvierteElPanicEnError(t *testing.T) {
	_, err := DescargarTodas(t.Context(), []string{"x"}, 1, func(context.Context, string) (string, error) { panic("ups") })
	if err == nil {
		t.Fatal("el pánico debería convertirse en error")
	}
}

func TestDescargarWG(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		res, err := DescargarWG(t.Context(), []string{"a", "b"}, fakeFetch)
		if err != nil || res[0] != "ok:a" || res[1] != "ok:b" {
			t.Fatalf("res=%v err=%v", res, err)
		}
		t0 := time.Now()
		_, err = DescargarWG(t.Context(), []string{"a", "mala"}, fakeFetch)
		if err == nil || time.Since(t0) != 10*time.Millisecond {
			t.Fatalf("err=%v tras %v", err, time.Since(t0))
		}
	})
}

func TestDescargarMalDevuelveConGoroutinesVivas(t *testing.T) {
	// Fuera de synctest: medimos el efecto real. Al volver, las demás descargas siguen en marcha.
	var terminadas atomic.Int32
	f := func(ctx context.Context, u string) (string, error) {
		if u == "mala" {
			return "", errors.New("falló")
		}
		time.Sleep(150 * time.Millisecond)
		terminadas.Add(1)
		return u, nil
	}
	_, err := DescargarMal([]string{"a", "b", "mala"}, f)
	if err == nil || terminadas.Load() != 0 {
		t.Fatalf("err=%v terminadas=%d; esperado error y 0 terminadas (siguen ejecutándose)", err, terminadas.Load())
	}
	time.Sleep(250 * time.Millisecond) // deja que acaben para que goleak no proteste
}
