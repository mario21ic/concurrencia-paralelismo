package main

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestTodosRecibenTodo(t *testing.T) {
	b := NuevoBroker[int]()
	a, c := b.Suscribir(context.Background(), 10), b.Suscribir(context.Background(), 10)
	for i := range 5 {
		if n := b.Publicar(i); n != 2 {
			t.Fatalf("entregados=%d, want 2", n)
		}
	}
	b.Cerrar()
	for _, s := range []*Suscripcion[int]{a, c} {
		n := 0
		for range s.C {
			n++
		}
		if n != 5 {
			t.Fatalf("recibió %d, want 5", n)
		}
	}
}

func TestLentoDescartaSinBloquear(t *testing.T) {
	b := NuevoBroker[int]()
	s := b.Suscribir(context.Background(), 2)
	hecho := make(chan struct{})
	go func() {
		for i := range 100 {
			b.Publicar(i) // nadie lee s.C: no debe bloquearse
		}
		close(hecho)
	}()
	select {
	case <-hecho:
	case <-time.After(time.Second):
		t.Fatal("Publicar se bloqueó por un suscriptor lento")
	}
	if s.Descartados() != 98 {
		t.Fatalf("descartados=%d, want 98", s.Descartados())
	}
}

func TestCancelarCierraCanal(t *testing.T) {
	b := NuevoBroker[int]()
	s := b.Suscribir(context.Background(), 1)
	s.Cancelar()
	s.Cancelar() // idempotente
	if _, ok := <-s.C; ok {
		t.Fatal("el canal debería estar cerrado")
	}
	if b.Suscriptores() != 0 {
		t.Fatal("debería haber 0 suscriptores")
	}
}

func TestContextoDaDeBaja(t *testing.T) {
	b := NuevoBroker[int]()
	ctx, cancel := context.WithCancel(context.Background())
	s := b.Suscribir(ctx, 1)
	cancel()
	select {
	case _, ok := <-s.C:
		if ok {
			t.Fatal("no esperaba datos")
		}
	case <-time.After(time.Second):
		t.Fatal("el contexto no dio de baja la suscripción")
	}
}

func TestSuscribirTrasCerrar(t *testing.T) {
	b := NuevoBroker[int]()
	b.Cerrar()
	s := b.Suscribir(context.Background(), 1)
	if _, ok := <-s.C; ok {
		t.Fatal("suscripción tras Cerrar debería devolver canal cerrado")
	}
}

// Estrés: publicar, suscribir y cancelar a la vez (usar -race).
func TestConcurrencia(t *testing.T) {
	b := NuevoBroker[int]()
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			for i := range 2000 {
				b.Publicar(i)
			}
		})
		wg.Go(func() {
			for range 200 {
				s := b.Suscribir(context.Background(), 4)
				select {
				case <-s.C:
				default:
				}
				s.Cancelar()
			}
		})
	}
	wg.Wait()
	b.Cerrar()
}
