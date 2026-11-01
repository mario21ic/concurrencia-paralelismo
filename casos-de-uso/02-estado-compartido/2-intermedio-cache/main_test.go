package main

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
)

func TestEstampidaSeUnificaEnUnaCarga(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var cargas atomic.Int32
		c := NuevaCache(time.Minute)
		var wg sync.WaitGroup
		for range 100 {
			wg.Go(func() {
				v, err := c.Obtener("k", func() (string, error) {
					cargas.Add(1)
					time.Sleep(100 * time.Millisecond)
					return "v", nil
				})
				if v != "v" || err != nil {
					t.Errorf("v=%q err=%v", v, err)
				}
			})
		}
		wg.Wait()
		if cargas.Load() != 1 {
			t.Fatalf("cargas = %d, esperado 1", cargas.Load())
		}
	})
}

func TestCaducaYSeRecarga(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var cargas int
		c := NuevaCache(10 * time.Second)
		carga := func() (string, error) { cargas++; return "v", nil }
		c.Obtener("k", carga)
		time.Sleep(9 * time.Second)
		c.Obtener("k", carga)
		if cargas != 1 {
			t.Fatalf("antes de caducar: cargas = %d", cargas)
		}
		time.Sleep(2 * time.Second)
		c.Obtener("k", carga)
		if cargas != 2 {
			t.Fatalf("tras caducar: cargas = %d, esperado 2", cargas)
		}
	})
}

func TestLosErroresNoSeCachean(t *testing.T) {
	c := NuevaCache(time.Minute)
	intentos := 0
	carga := func() (string, error) {
		intentos++
		if intentos == 1 {
			return "", errors.New("BD caída")
		}
		return "ok", nil
	}
	if _, err := c.Obtener("k", carga); err == nil {
		t.Fatal("el primer intento debía fallar")
	}
	if v, err := c.Obtener("k", carga); err != nil || v != "ok" {
		t.Fatalf("el segundo debía reintentar: %q %v", v, err)
	}
}

func TestClavesDistintasNoSeBloqueanEntreSi(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := NuevaCache(time.Minute)
		var wg sync.WaitGroup
		t0 := time.Now()
		for _, k := range []string{"a", "b", "c"} {
			wg.Go(func() {
				c.Obtener(k, func() (string, error) { time.Sleep(100 * time.Millisecond); return k, nil })
			})
		}
		wg.Wait()
		if d := time.Since(t0); d != 100*time.Millisecond {
			t.Fatalf("tardó %v: las cargas de claves distintas deben solaparse", d)
		}
	})
}
