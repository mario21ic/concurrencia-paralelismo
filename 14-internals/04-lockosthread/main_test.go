package main

import (
	"runtime"
	"sync"
	"testing"
)

func TestFijadaSiempreMismoHilo(t *testing.T) {
	if !tidDisponible {
		t.Skip("sin identificador de hilo en este SO")
	}
	con := hilosVistos(true)
	if len(con) != 1 {
		t.Fatalf("con LockOSThread la goroutine usó %d hilos, want 1", len(con))
	}
}

func TestHiloPrincipalAtiendeLasLlamadas(t *testing.T) {
	// En un test la goroutine de test NO es el hilo principal, así que se simula el bucle
	// en una goroutine fijada y se comprueba que TODAS las llamadas caen en el mismo hilo.
	if !tidDisponible {
		t.Skip()
	}
	parar := make(chan struct{})
	hiloBucle := make(chan int, 1)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		hiloBucle <- tid()
		BucleDelHiloPrincipal(parar)
	}()
	esperado := <-hiloBucle

	var wg sync.WaitGroup
	var mu sync.Mutex
	var vistos []int
	for range 20 {
		wg.Go(func() {
			EnHiloPrincipal(func() {
				mu.Lock()
				vistos = append(vistos, tid())
				mu.Unlock()
			})
		})
	}
	wg.Wait()
	close(parar)
	for _, h := range vistos {
		if h != esperado {
			t.Fatalf("una llamada se ejecutó en el hilo %d, want %d", h, esperado)
		}
	}
	if len(vistos) != 20 {
		t.Fatalf("llamadas atendidas: %d", len(vistos))
	}
}

func BenchmarkPingPongNormal(b *testing.B) {
	for b.Loop() {
		pingPong(1000, false)
	}
}

func BenchmarkPingPongFijado(b *testing.B) {
	for b.Loop() {
		pingPong(1000, true)
	}
}

func TestTerminarSinSoltarDestruyeElHilo(t *testing.T) {
	if !tidDisponible {
		t.Skip()
	}
	sin := hilosDistintos(50, false)
	con := hilosDistintos(50, true)
	if sin != 50 {
		t.Fatalf("sin soltar: %d hilos distintos, want 50 (uno nuevo por goroutine)", sin)
	}
	if con >= 10 {
		t.Fatalf("soltando: %d hilos distintos; se esperaba reutilización (pocos)", con)
	}
}
