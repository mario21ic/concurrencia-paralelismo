package main

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"testing"
	"time"
)

// reloj falso: avanza solo cuando el test lo dice.
type reloj struct {
	mu sync.Mutex
	t  time.Time
}

func (r *reloj) Now() time.Time {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.t
}
func (r *reloj) Avanzar(d time.Duration) {
	r.mu.Lock()
	r.t = r.t.Add(d)
	r.mu.Unlock()
}

func nueva(t *testing.T) (*Cache[string, int], *reloj) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	c := Nueva[string, int](ctx, 0) // sin barredor: el test controla el tiempo
	r := &reloj{t: time.Unix(1_000_000, 0)}
	c.ahora = r.Now
	return c, r
}

func TestSetGet(t *testing.T) {
	c, _ := nueva(t)
	c.Set("a", 1, time.Minute)
	if v, ok := c.Get("a"); !ok || v != 1 {
		t.Fatalf("Get(a) = %d,%v", v, ok)
	}
	if _, ok := c.Get("zz"); ok {
		t.Fatal("Get de clave ausente debería fallar")
	}
}

func TestExpiracionPerezosa(t *testing.T) {
	c, r := nueva(t)
	c.Set("a", 1, 10*time.Second)
	r.Avanzar(9 * time.Second)
	if _, ok := c.Get("a"); !ok {
		t.Fatal("a 9s: debería seguir viva")
	}
	r.Avanzar(1 * time.Second) // justo en el límite: vencida
	if _, ok := c.Get("a"); ok {
		t.Fatal("a 10s: debería haber vencido")
	}
	if c.Len() != 0 {
		t.Fatalf("Len = %d; Get debe eliminar la vencida", c.Len())
	}
	if s := c.Stats(); s.Expiradas != 1 {
		t.Fatalf("Expiradas = %d, want 1", s.Expiradas)
	}
}

func TestSinTTLNoExpira(t *testing.T) {
	c, r := nueva(t)
	c.Set("eterna", 7, 0)
	r.Avanzar(1000 * time.Hour)
	if v, ok := c.Get("eterna"); !ok || v != 7 {
		t.Fatal("una entrada sin TTL no debe expirar")
	}
}

func TestBarrer(t *testing.T) {
	c, r := nueva(t)
	for i := range 10 {
		c.Set(fmt.Sprint("c", i), i, time.Second)
	}
	c.Set("larga", 1, time.Hour)
	r.Avanzar(2 * time.Second)
	if n := c.Barrer(); n != 10 {
		t.Fatalf("Barrer = %d, want 10", n)
	}
	if c.Len() != 1 {
		t.Fatalf("Len = %d, want 1", c.Len())
	}
}

func TestSobrescribirRenuevaTTL(t *testing.T) {
	c, r := nueva(t)
	c.Set("a", 1, 10*time.Second)
	r.Avanzar(8 * time.Second)
	c.Set("a", 2, 10*time.Second) // renueva
	r.Avanzar(8 * time.Second)
	if v, ok := c.Get("a"); !ok || v != 2 {
		t.Fatalf("Get = %d,%v; la entrada renovada debería seguir viva", v, ok)
	}
}

func TestDelete(t *testing.T) {
	c, _ := nueva(t)
	c.Set("a", 1, 0)
	if !c.Delete("a") || c.Delete("a") {
		t.Fatal("Delete debe devolver true la primera vez y false la segunda")
	}
}

func TestStats(t *testing.T) {
	c, _ := nueva(t)
	c.Set("a", 1, 0)
	c.Get("a")
	c.Get("a")
	c.Get("b")
	if s := c.Stats(); s.Aciertos != 2 || s.Fallos != 1 {
		t.Fatalf("Stats = %+v", s)
	}
}

// El barredor real elimina entradas y termina al cancelar el contexto.
func TestBarredorYCancelacion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	c := Nueva[string, int](ctx, 5*time.Millisecond)
	c.Set("a", 1, 10*time.Millisecond)

	limite := time.Now().Add(2 * time.Second)
	for c.Len() != 0 {
		if time.Now().After(limite) {
			t.Fatal("el barredor no eliminó la entrada vencida")
		}
		time.Sleep(2 * time.Millisecond)
	}

	cancel()
	select {
	case <-c.Hecho():
	case <-time.After(2 * time.Second):
		t.Fatal("el barredor no terminó al cancelar el contexto")
	}
}

func TestSinFugaDeGoroutines(t *testing.T) {
	antes := runtime.NumGoroutine()
	for range 50 {
		ctx, cancel := context.WithCancel(context.Background())
		c := Nueva[int, int](ctx, time.Millisecond)
		cancel()
		<-c.Hecho()
	}
	time.Sleep(20 * time.Millisecond)
	if despues := runtime.NumGoroutine(); despues > antes+2 {
		t.Fatalf("goroutines: antes=%d después=%d", antes, despues)
	}
}

// Prueba de estrés: ejecutar con -race.
func TestConcurrencia(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := Nueva[int, int](ctx, time.Millisecond)

	var wg sync.WaitGroup
	for g := range 16 {
		wg.Go(func() {
			for i := range 5000 {
				k := (g*31 + i) % 64
				switch i % 5 {
				case 0:
					c.Set(k, i, time.Millisecond)
				case 1:
					c.Delete(k)
				case 2:
					c.Barrer()
				default:
					c.Get(k)
				}
			}
		})
	}
	wg.Wait()
	s := c.Stats()
	if s.Aciertos+s.Fallos == 0 {
		t.Fatal("no se contabilizaron lecturas")
	}
}

func BenchmarkGetParalelo(b *testing.B) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c := Nueva[int, int](ctx, 0)
	for i := range 1024 {
		c.Set(i, i, time.Hour)
	}
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			c.Get(i & 1023)
			i++
		}
	})
}
