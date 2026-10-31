package main

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func politicas() map[string]func() RW {
	return map[string]func() RW{
		"pref-lectores":   func() RW { return new(PrefLectores) },
		"pref-escritores": func() RW { return NuevoPrefEscritores() },
		"justo":           func() RW { return new(Justo) },
		"estandar":        func() RW { return new(Estandar) },
	}
}

// Exclusión: nunca hay un escritor junto a otro escritor o a un lector (con -race, además, el dato
// compartido es un int normal: si el cerrojo fallara, el detector lo diría).
func TestExclusion(t *testing.T) {
	for nombre, nuevo := range politicas() {
		t.Run(nombre, func(t *testing.T) {
			l := nuevo()
			var lectores, escritores atomic.Int32
			var dato int
			var wg sync.WaitGroup
			for range 6 {
				wg.Go(func() {
					for range 300 {
						l.RLock()
						lectores.Add(1)
						if escritores.Load() != 0 {
							t.Error("lector con un escritor dentro")
						}
						_ = dato
						lectores.Add(-1)
						l.RUnlock()
					}
				})
			}
			for range 3 {
				wg.Go(func() {
					for range 100 {
						l.Lock()
						if escritores.Add(1) != 1 || lectores.Load() != 0 {
							t.Error("escritor no exclusivo")
						}
						dato++
						escritores.Add(-1)
						l.Unlock()
					}
				})
			}
			wg.Wait()
			if dato != 300 {
				t.Fatalf("dato = %d, esperado 300", dato)
			}
		})
	}
}

// escritorProgresa dice si un escritor consigue entrar en 300 ms con lectores continuos solapados.
func escritorProgresa(l RW) bool {
	var parar atomic.Bool
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for !parar.Load() {
				l.RLock()
				time.Sleep(200 * time.Microsecond)
				l.RUnlock()
			}
		})
	}
	time.Sleep(5 * time.Millisecond)
	entro := make(chan struct{})
	go func() { l.Lock(); l.Unlock(); close(entro) }()
	defer func() { parar.Store(true); wg.Wait() }()
	select {
	case <-entro:
		return true
	case <-time.After(300 * time.Millisecond):
		return false
	}
}

func TestInanicionDelEscritor(t *testing.T) {
	if escritorProgresa(new(PrefLectores)) {
		t.Fatal("con preferencia de lectores el escritor no debería entrar con lectores solapados")
	}
	for _, c := range []struct {
		n string
		l RW
	}{{"pref-escritores", NuevoPrefEscritores()}, {"justo", new(Justo)}, {"estandar", new(Estandar)}} {
		if !escritorProgresa(c.l) {
			t.Fatalf("%s: el escritor sufrió inanición", c.n)
		}
	}
}
