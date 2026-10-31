package main

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sync/singleflight"
)

// Se libera la ejecución solo cuando los 50 llamadores ya están dentro de Do.
func TestUnaSolaEjecucion(t *testing.T) {
	var sf singleflight.Group
	var ejecuciones, llegaron atomic.Int32
	liberar := make(chan struct{})
	hecho := make(chan struct{})
	go func() {
		lanzar(50, func(int) {
			llegaron.Add(1)
			sf.Do("k", func() (any, error) {
				ejecuciones.Add(1)
				<-liberar
				return 1, nil
			})
		})
		close(hecho)
	}()
	for llegaron.Load() < 50 {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond) // deja que todos entren en Do
	close(liberar)
	<-hecho
	if ejecuciones.Load() != 1 {
		t.Fatalf("ejecuciones=%d, want 1", ejecuciones.Load())
	}
}

func TestSecuencialesNoSeAgrupan(t *testing.T) {
	var sf singleflight.Group
	n := 0
	for range 3 {
		sf.Do("k", func() (any, error) { n++; return n, nil })
	}
	if n != 3 {
		t.Fatalf("n=%d, want 3", n)
	}
}

func TestDoChanCancelacionPorLlamador(t *testing.T) {
	var db baseDeDatos
	var sf singleflight.Group
	obtener := func(ctx context.Context) error {
		ch := sf.DoChan("k", func() (any, error) { return db.Obtener(context.WithoutCancel(ctx), "k") })
		select {
		case r := <-ch:
			return r.Err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	corto, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()
	var e1 error
	hecho := make(chan struct{})
	go func() { e1 = obtener(corto); close(hecho) }()
	time.Sleep(time.Millisecond)
	if err := obtener(context.Background()); err != nil {
		t.Fatalf("el llamador paciente no debería fallar: %v", err)
	}
	<-hecho
	if e1 == nil {
		t.Fatal("el impaciente debería haber abandonado")
	}
	if db.consultas.Load() != 1 {
		t.Fatalf("consultas=%d, want 1", db.consultas.Load())
	}
}
