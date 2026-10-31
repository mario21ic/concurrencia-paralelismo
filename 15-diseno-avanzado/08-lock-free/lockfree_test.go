package main

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestABASinEtiquetaCorrompe(t *testing.T) {
	cas, antes, despues := EscenarioABA(false)
	t.Logf("antes %s, después %s", antes, despues)
	if !cas {
		t.Fatal("sin etiqueta el CAS debería (erróneamente) triunfar")
	}
	if despues != "[B → C]" {
		t.Fatalf("esperaba la pila corrupta [B → C] (B resucitado), hay %s", despues)
	}
}

func TestABAConEtiquetaLoEvita(t *testing.T) {
	cas, _, despues := EscenarioABA(true)
	if cas {
		t.Fatal("con etiqueta el CAS debe fallar")
	}
	if despues != "[X → C]" {
		t.Fatalf("la pila debe seguir intacta ([X → C]), hay %s", despues)
	}
}

// Conservación: lo que entra sale exactamente una vez (suma y cuenta).
func estres(t *testing.T, push func(int64), pop func() (int64, bool)) {
	t.Helper()
	const prod, cada = 4, 20_000
	var suma, cuenta atomic.Int64
	var wp, wc sync.WaitGroup
	var fin atomic.Bool
	for p := range prod {
		wp.Go(func() {
			for i := range cada {
				push(int64(p*cada + i + 1))
			}
		})
	}
	for range 4 {
		wc.Go(func() {
			for {
				if v, ok := pop(); ok {
					suma.Add(v)
					cuenta.Add(1)
				} else if fin.Load() {
					if v, ok := pop(); ok {
						suma.Add(v)
						cuenta.Add(1)
						continue
					}
					return
				}
			}
		})
	}
	wp.Wait()
	fin.Store(true)
	wc.Wait()
	n := int64(prod * cada)
	if cuenta.Load() != n || suma.Load() != n*(n+1)/2 {
		t.Fatalf("cuenta=%d suma=%d; esperado %d y %d", cuenta.Load(), suma.Load(), n, n*(n+1)/2)
	}
}

func TestPilaTreiber(t *testing.T) {
	var p PilaTreiber[int64]
	estres(t, p.Push, p.Pop)
}

func TestColaMS(t *testing.T) {
	c := NuevaColaMS[int64]()
	estres(t, c.Enqueue, c.Dequeue)
}

func TestPilaIdxConEtiqueta(t *testing.T) {
	// El pool tiene tantos huecos como elementos en vuelo como máximo: 4 productores x 1 + margen.
	p := NuevaPilaIdx(4096, true)
	const g, vueltas = 4, 20_000
	var wg sync.WaitGroup
	var suma atomic.Int64
	for id := range g {
		wg.Go(func() {
			for i := range vueltas {
				p.Push(int64(id*vueltas + i + 1))
				v, ok := p.Pop()
				if !ok {
					t.Error("Pop vacío tras un Push propio")
					return
				}
				suma.Add(v)
			}
		})
	}
	wg.Wait()
	n := int64(g * vueltas)
	if suma.Load() != n*(n+1)/2 {
		t.Fatalf("suma = %d, esperado %d", suma.Load(), n*(n+1)/2)
	}
	if _, ok := p.Pop(); ok {
		t.Fatal("la pila debería estar vacía")
	}
}

func TestColaMSFIFOSecuencial(t *testing.T) {
	c := NuevaColaMS[int]()
	for i := range 100 {
		c.Enqueue(i)
	}
	for i := range 100 {
		if v, ok := c.Dequeue(); !ok || v != i {
			t.Fatalf("Dequeue = %d,%v; esperado %d", v, ok, i)
		}
	}
	if _, ok := c.Dequeue(); ok {
		t.Fatal("debería estar vacía")
	}
}

func TestContadores(t *testing.T) {
	var m ContadorMutex
	var c ContadorCAS
	var a ContadorAdd
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 10_000 {
				m.Inc()
				c.Inc()
				a.Inc()
			}
		})
	}
	wg.Wait()
	if m.n != 80_000 || c.n.Load() != 80_000 || a.n.Load() != 80_000 {
		t.Fatalf("%d %d %d", m.n, c.n.Load(), a.n.Load())
	}
}
