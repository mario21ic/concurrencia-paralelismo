package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/synctest"
	"time"
)

func TestRafagaYLuegoTasa(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := NuevoLimitador(1, 3) // 1/s, ráfaga 3
		for i := 1; i <= 3; i++ {
			if ok, _ := l.Permitir("a"); !ok {
				t.Fatalf("la petición %d de la ráfaga debía pasar", i)
			}
		}
		ok, espera := l.Permitir("a")
		if ok || espera != time.Second {
			t.Fatalf("4.ª: ok=%v espera=%v, esperado rechazo con espera exacta de 1 s", ok, espera)
		}
		time.Sleep(espera)
		if ok, _ := l.Permitir("a"); !ok {
			t.Fatal("tras esperar lo indicado debía pasar")
		}
	})
}

func TestUnRechazoNoGastaFicha(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := NuevoLimitador(1, 1)
		l.Permitir("a")
		for range 10 { // insistir no debe empeorar la espera
			if ok, d := l.Permitir("a"); ok || d != time.Second {
				t.Fatalf("ok=%v espera=%v", ok, d)
			}
		}
	})
}

func TestClientesIndependientes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := NuevoLimitador(1, 1)
		l.Permitir("abusivo")
		if ok, _ := l.Permitir("abusivo"); ok {
			t.Fatal("el abusivo debía quedar limitado")
		}
		if ok, _ := l.Permitir("tranquilo"); !ok {
			t.Fatal("otro cliente no debe verse afectado")
		}
	})
}

func TestPodaDeOciosos(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := NuevoLimitador(1, 1)
		l.Permitir("a")
		time.Sleep(30 * time.Second)
		l.Permitir("b")
		time.Sleep(40 * time.Second)
		if n := l.Podar(time.Minute); n != 1 || l.Clientes() != 1 { // «a» lleva 70 s; «b», 40 s
			t.Fatalf("eliminados=%d quedan=%d", n, l.Clientes())
		}
	})
}

func TestMiddlewareDevuelve429ConRetryAfter(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := NuevoLimitador(0.5, 1) // una ficha cada 2 s
		h := l.Middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
		pedir := func() *httptest.ResponseRecorder {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("X-Cliente", "x")
			h.ServeHTTP(rec, req)
			return rec
		}
		if r := pedir(); r.Code != 200 {
			t.Fatalf("1.ª: %d", r.Code)
		}
		if r := pedir(); r.Code != http.StatusTooManyRequests || r.Header().Get("Retry-After") != "2" {
			t.Fatalf("2.ª: %d Retry-After=%q", r.Code, r.Header().Get("Retry-After"))
		}
	})
}
