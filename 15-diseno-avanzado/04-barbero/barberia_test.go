package main

import (
	"context"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

// llegadas lanza n clientes, uno cada `cada`, y espera a que todos terminen.
func llegadas(b *Barberia, n int, cada time.Duration) {
	var wg sync.WaitGroup
	for i := range n {
		if i > 0 {
			time.Sleep(cada)
		}
		wg.Go(func() { b.Llega() })
	}
	wg.Wait()
}

func TestCapacidadExacta(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		b := Abrir(ctx, 1, 2, 10*time.Millisecond)
		// t=0 entra directo al barbero; t=1 y t=2 ocupan las 2 sillas; t=3..9 encuentran todo lleno.
		llegadas(b, 10, time.Millisecond)
		cancel()
		b.Cerrar()
		if b.Atendidos.Load() != 3 || b.Perdidos.Load() != 7 {
			t.Fatalf("atendidos=%d perdidos=%d, esperado 3 y 7", b.Atendidos.Load(), b.Perdidos.Load())
		}
	})
}

func TestCeroSillasSoloEntraSiElBarberoDuerme(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		b := Abrir(ctx, 1, 0, 10*time.Millisecond)
		synctest.Wait()                    // el barbero está dormido (bloqueado en la recepción)
		llegadas(b, 3, 4*time.Millisecond) // el 1.º lo despierta; el 2.º y 3.º lo encuentran cortando
		cancel()
		b.Cerrar()
		if b.Atendidos.Load() != 1 || b.Perdidos.Load() != 2 {
			t.Fatalf("atendidos=%d perdidos=%d, esperado 1 y 2", b.Atendidos.Load(), b.Perdidos.Load())
		}
	})
}

func TestConservacionDeClientes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		b := Abrir(ctx, 2, 3, 7*time.Millisecond)
		llegadas(b, 200, 2*time.Millisecond)
		cancel()
		b.Cerrar()
		if got := b.Atendidos.Load() + b.Perdidos.Load(); got != 200 {
			t.Fatalf("atendidos+perdidos = %d, esperado 200", got)
		}
	})
}

func TestMasBarberosPierdenMenos(t *testing.T) {
	perdidos := func(barberos int) int64 {
		var p int64
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			b := Abrir(ctx, barberos, 1, 10*time.Millisecond)
			llegadas(b, 100, 3*time.Millisecond)
			cancel()
			b.Cerrar()
			p = b.Perdidos.Load()
		})
		return p
	}
	p1, p2, p4 := perdidos(1), perdidos(2), perdidos(4)
	t.Logf("perdidos con 1, 2 y 4 barberos: %d, %d, %d", p1, p2, p4)
	if !(p1 > p2 && p2 > p4) {
		t.Fatal("más barberos deberían perder menos clientes")
	}
}
