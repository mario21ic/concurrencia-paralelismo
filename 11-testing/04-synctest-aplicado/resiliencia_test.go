package resiliencia

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

// ---- Reintentar: el backoff completo (100+200+400 = 700 ms) sin esperar nada ----

func TestReintentarSeRecupera(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		llamadas := 0
		ini := time.Now()
		err := Reintentar(t.Context(), 5, 100*time.Millisecond, func(context.Context) error {
			if llamadas++; llamadas < 3 {
				return ErrTransitorio
			}
			return nil
		})
		if err != nil {
			t.Fatalf("err = %v", err)
		}
		// esperó 100 ms tras el 1.er fallo y 200 ms tras el 2.º
		if got := time.Since(ini); got != 300*time.Millisecond {
			t.Fatalf("duración = %v, want 300ms", got)
		}
		if llamadas != 3 {
			t.Fatalf("llamadas = %d, want 3", llamadas)
		}
	})
}

func TestReintentarAgotaIntentos(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ini := time.Now()
		err := Reintentar(t.Context(), 4, 100*time.Millisecond, func(context.Context) error { return ErrTransitorio })
		if !errors.Is(err, ErrTransitorio) {
			t.Fatalf("err = %v", err)
		}
		if got := time.Since(ini); got != 700*time.Millisecond { // 100+200+400
			t.Fatalf("duración = %v, want 700ms", got)
		}
	})
}

func TestReintentarBackoffGrande(t *testing.T) {
	// 10 intentos con base de 1 minuto: ¡más de 17 horas de espera! En tiempo real
	// sería imposible de probar; aquí tarda microsegundos.
	synctest.Test(t, func(t *testing.T) {
		ini := time.Now()
		Reintentar(t.Context(), 10, time.Minute, func(context.Context) error { return ErrTransitorio })
		want := (1<<9 - 1) * time.Minute // 1+2+4+...+256 = 511 minutos
		if got := time.Since(ini); got != want {
			t.Fatalf("duración = %v, want %v", got, want)
		}
	})
}

func TestReintentarCancelacion(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithTimeout(t.Context(), 250*time.Millisecond)
		defer cancel()
		ini := time.Now()
		err := Reintentar(ctx, 10, 100*time.Millisecond, func(context.Context) error { return ErrTransitorio })
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err = %v, want DeadlineExceeded", err)
		}
		if got := time.Since(ini); got != 250*time.Millisecond {
			t.Fatalf("se detuvo a los %v, want 250ms exactos", got)
		}
	})
}

// ---- Debouncer ----

func TestDebounceAgrupaRafagas(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var disparos atomic.Int32
		b := NuevoDebouncer(100 * time.Millisecond)
		for range 5 { // 5 llamadas separadas 50 ms: cada una reinicia la cuenta atrás
			b.Llamar(func() { disparos.Add(1) })
			time.Sleep(50 * time.Millisecond)
		}
		synctest.Wait()
		if disparos.Load() != 0 {
			t.Fatalf("disparos = %d antes de 100ms de silencio", disparos.Load())
		}
		time.Sleep(50 * time.Millisecond) // completa 100 ms desde la última llamada
		synctest.Wait()
		if disparos.Load() != 1 {
			t.Fatalf("disparos = %d, want 1", disparos.Load())
		}
	})
}

func TestDebounceLlamadasEspaciadas(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var disparos atomic.Int32
		b := NuevoDebouncer(100 * time.Millisecond)
		for range 3 {
			b.Llamar(func() { disparos.Add(1) })
			time.Sleep(150 * time.Millisecond) // más que d: cada una se dispara
		}
		synctest.Wait()
		if disparos.Load() != 3 {
			t.Fatalf("disparos = %d, want 3", disparos.Load())
		}
	})
}

// ---- CacheTTL ----

func TestCacheExpiraExactamenteEnElTTL(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := NuevaCacheTTL[string, int](t.Context(), time.Hour*24)
		c.Set("a", 1, time.Hour)

		time.Sleep(time.Hour - time.Nanosecond)
		if _, ok := c.Get("a"); !ok {
			t.Fatal("a 59m59.999999999s debería seguir viva")
		}
		time.Sleep(time.Nanosecond)
		if _, ok := c.Get("a"); ok {
			t.Fatal("a 1h exacta debería haber vencido")
		}
	})
}

func TestBarredorEliminaSinGet(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := NuevaCacheTTL[int, int](t.Context(), time.Minute)
		for i := range 100 {
			c.Set(i, i, 5*time.Minute)
		}
		c.Set(-1, -1, time.Hour)

		time.Sleep(4 * time.Minute)
		synctest.Wait()
		if c.Len() != 101 {
			t.Fatalf("Len = %d a los 4 min, want 101", c.Len())
		}
		time.Sleep(2 * time.Minute) // el barrido de los 5 min elimina las 100 vencidas
		synctest.Wait()
		if c.Len() != 1 {
			t.Fatalf("Len = %d a los 6 min, want 1", c.Len())
		}
	})
}

// Al terminar la burbuja el barredor DEBE haber salido (t.Context() se cancela
// antes de Cleanup). Si no lo hiciera, synctest.Test entraría en pánico por fuga.
func TestBarredorTerminaConElContexto(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		NuevaCacheTTL[int, int](t.Context(), time.Second)
		time.Sleep(10 * time.Second)
	})
}
