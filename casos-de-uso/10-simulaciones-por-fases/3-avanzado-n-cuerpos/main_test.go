package main

import (
	"math"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
)

func TestBarreraCiclica(t *testing.T) {
	// En cada ronda r, tras la barrera TODOS los incrementos de la ronda r ya deben haberse hecho.
	const k, rondas = 6, 200
	b := NuevaBarrera(k)
	var cuenta atomic.Int64
	var wg sync.WaitGroup
	fallos := make(chan string, k*rondas)
	for range k {
		wg.Go(func() {
			for r := 1; r <= rondas; r++ {
				cuenta.Add(1)
				b.Esperar()
				if got := cuenta.Load(); got < int64(r*k) {
					fallos <- "alguien cruzó la barrera antes de tiempo"
				}
				b.Esperar() // segunda barrera: evita que un rápido incremente la ronda r+1 antes de comprobar
			}
		})
	}
	wg.Wait()
	close(fallos)
	for f := range fallos {
		t.Error(f)
	}
	if cuenta.Load() != k*rondas {
		t.Fatalf("cuenta = %d", cuenta.Load())
	}
}

func TestBarreraBloqueaHastaElUltimo(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		b := NuevaBarrera(3)
		var pasaron atomic.Int32
		for range 2 {
			go func() { b.Esperar(); pasaron.Add(1) }()
		}
		synctest.Wait() // los 2 primeros quedan bloqueados en Esperar
		if pasaron.Load() != 0 {
			t.Fatal("no debería pasar nadie hasta que llegue el tercero")
		}
		b.Esperar() // el tercero abre la barrera
		synctest.Wait()
		if pasaron.Load() != 2 {
			t.Fatalf("pasaron %d, esperado 2", pasaron.Load())
		}
	})
}

func TestTodasLasVersionesDanElMismoResultado(t *testing.T) {
	orig := Nuevos(96, 3)
	want := orig.Clon()
	Secuencial(want, 30)
	for nombre, f := range map[string]func(*Cuerpos, int, int){"por fase": PorFase, "persistentes": Persistentes} {
		for _, k := range []int{1, 2, 3, 8, 200} {
			got := orig.Clon()
			f(got, 30, k)
			if !slices.Equal(got.x, want.x) || !slices.Equal(got.y, want.y) || !slices.Equal(got.vx, want.vx) {
				t.Errorf("%s k=%d: distinto del secuencial", nombre, k)
			}
		}
	}
}

func TestSeConservaElMomento(t *testing.T) {
	c := Nuevos(64, 5)
	px0, py0 := c.Momento()
	Persistentes(c, 100, 4)
	px, py := c.Momento()
	if math.Abs(px-px0) > 1e-9 || math.Abs(py-py0) > 1e-9 {
		t.Fatalf("el momento cambió: (%v, %v) → (%v, %v)", px0, py0, px, py)
	}
}
