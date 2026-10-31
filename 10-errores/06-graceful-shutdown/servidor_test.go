package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

func arrancar(t *testing.T, gracia time.Duration, h http.Handler) (url string, cancel context.CancelFunc, hecho <-chan error, latidos *atomic.Int64) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	latidos = new(atomic.Int64)
	ch := make(chan error, 1)
	go func() { ch <- Ejecutar(ctx, ln, h, gracia, latidos) }()
	return "http://" + ln.Addr().String(), cancel, ch, latidos
}

func TestPeticionEnCursoTerminaTrasElApagado(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(300 * time.Millisecond)
		io.WriteString(w, "completa")
	})
	url, cancel, hecho, _ := arrancar(t, 5*time.Second, h)

	resp := make(chan string, 1)
	go func() {
		r, err := http.Get(url)
		if err != nil {
			resp <- "error: " + err.Error()
			return
		}
		defer r.Body.Close()
		b, _ := io.ReadAll(r.Body)
		resp <- string(b)
	}()
	time.Sleep(100 * time.Millisecond) // la petición ya está en curso
	cancel()                           // pedir apagado

	if got := <-resp; got != "completa" {
		t.Fatalf("la petición en curso debía completarse, got %q", got)
	}
	if err := <-hecho; err != nil {
		t.Fatalf("Ejecutar = %v, want nil", err)
	}
	if _, err := http.Get(url); err == nil {
		t.Fatal("tras el apagado no debe aceptar conexiones")
	}
}

func TestGraciaAgotadaCortaConexiones(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(5 * time.Second):
		case <-r.Context().Done():
		}
	})
	url, cancel, hecho, _ := arrancar(t, 100*time.Millisecond, h)

	errCliente := make(chan error, 1)
	go func() {
		_, err := http.Get(url)
		errCliente <- err
	}()
	time.Sleep(100 * time.Millisecond)
	ini := time.Now()
	cancel()

	if err := <-hecho; err == nil {
		t.Fatal("con gracia agotada Ejecutar debería devolver error")
	}
	if time.Since(ini) > 2*time.Second {
		t.Fatal("el apagado tardó demasiado")
	}
	if err := <-errCliente; err == nil {
		t.Fatal("el cliente debería recibir un error al cortarse la conexión")
	}
}

func TestWorkerDeFondoSeDetiene(t *testing.T) {
	url, cancel, hecho, latidos := arrancar(t, time.Second, manejador())
	if _, err := http.Get(url + "/ping"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(60 * time.Millisecond)
	cancel()
	if err := <-hecho; err != nil {
		t.Fatal(err)
	}
	n := latidos.Load()
	time.Sleep(50 * time.Millisecond)
	if latidos.Load() != n {
		t.Fatal("el worker siguió latiendo tras el apagado")
	}
	if n == 0 {
		t.Fatal("el worker nunca latió")
	}
}

func TestFalloDelListenerCancelaAlResto(t *testing.T) {
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	ln.Close() // Serve fallará de inmediato
	var latidos atomic.Int64
	hecho := make(chan error, 1)
	go func() { hecho <- Ejecutar(context.Background(), ln, manejador(), time.Second, &latidos) }()
	select {
	case err := <-hecho:
		if err == nil {
			t.Fatal("se esperaba error del listener")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("el fallo del servidor no detuvo a los demás componentes")
	}
}
