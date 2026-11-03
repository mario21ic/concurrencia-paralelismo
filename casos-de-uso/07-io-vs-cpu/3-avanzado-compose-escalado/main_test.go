package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestGastaEsDeterminista(t *testing.T) {
	if Gasta(1000, 3) != Gasta(1000, 3) || Gasta(1000, 3) == Gasta(1000, 4) {
		t.Fatal("Gasta debe ser determinista y depender de la semilla")
	}
}

func TestManejador(t *testing.T) {
	srv := httptest.NewServer(Manejador("w1"))
	defer srv.Close()
	r, err := peticion(context.Background(), srv.Client(), srv.URL+"/trabajo?iter=1000&semilla=5")
	if err != nil {
		t.Fatal(err)
	}
	if r.worker != "w1" || r.valor != Gasta(1000, 5) {
		t.Fatalf("respuesta inesperada: %+v", r)
	}
	if _, err := peticion(context.Background(), srv.Client(), srv.URL+"/trabajo?iter=x"); err == nil {
		t.Fatal("una petición mal formada debe fallar")
	}
}

func TestEjecutarRepartePeticionesYResultado(t *testing.T) {
	srv := httptest.NewServer(Manejador("w1"))
	defer srv.Close()
	cfg := Config{URL: srv.URL, Tareas: 50, Iter: 20_000, Conc: 8}
	res, err := Ejecutar(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.PorWorker["w1"] != 50 {
		t.Fatalf("respondieron %v, esperado 50 de w1", res.PorWorker)
	}
	var want uint64
	want ^= Gasta(0, 1)
	for i := range 50 {
		want ^= Gasta(20_000, uint64(i+2))
	}
	if res.Xor != want {
		t.Fatalf("XOR %x, esperado %x", res.Xor, want)
	}
}

func TestLaParteSerialSeQuitaDeCadaTarea(t *testing.T) {
	var mu sync.Mutex
	var iters []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		iters = append(iters, r.URL.Query().Get("iter"))
		mu.Unlock()
		w.Header().Set("X-Worker", "w")
		w.Write([]byte("1"))
	}))
	defer srv.Close()
	res, err := Ejecutar(context.Background(), Config{URL: srv.URL, Tareas: 10, Iter: 1000, Conc: 2, Serial: 0.25})
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range iters {
		if it != "750" {
			t.Fatalf("con 25 %% serial cada tarea debe llevar 750 iteraciones, llegó %s", it)
		}
	}
	if len(iters) != 10 || res.Serial <= 0 {
		t.Fatalf("peticiones=%d serial=%v", len(iters), res.Serial)
	}
}

func TestEjecutarDevuelveErrorSiElWorkerFalla(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "caído", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	if _, err := Ejecutar(context.Background(), Config{URL: srv.URL, Tareas: 5, Iter: 10, Conc: 2}); err == nil {
		t.Fatal("debe devolver error")
	}
}
