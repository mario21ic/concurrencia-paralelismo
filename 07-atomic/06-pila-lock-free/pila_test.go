package main

import "testing"

func TestPilaLIFO(t *testing.T) {
	var p Pila[int]
	if _, ok := p.Pop(); ok {
		t.Fatal("pila vacía debería devolver ok=false")
	}
	for i := 1; i <= 3; i++ {
		p.Push(i)
	}
	for want := 3; want >= 1; want-- {
		got, ok := p.Pop()
		if !ok || got != want {
			t.Fatalf("Pop = %d,%v; want %d", got, ok, want)
		}
	}
}

func TestPilaConcurrente(t *testing.T) {
	s, e, _ := prueba(&Pila[int]{}, 8, 8, 20_000)
	if s != e {
		t.Fatalf("suma=%d esperada=%d (se perdieron o duplicaron elementos)", s, e)
	}
}

func BenchmarkLockFree(b *testing.B) {
	var p Pila[int]
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			p.Push(1)
			p.Pop()
		}
	})
}

func BenchmarkMutexPila(b *testing.B) {
	var p pilaMutex[int]
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			p.Push(1)
			p.Pop()
		}
	})
}
