package main

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m, goleak.IgnoreAnyFunction("net/http.(*persistConn).readLoop"), goleak.IgnoreAnyFunction("net/http.(*persistConn).writeLoop"))
}

// lanzar hace n GET concurrentes y devuelve cuántos recibieron cada código.
func lanzar(t *testing.T, url string, n int) (codigos map[int]int) {
	t.Helper()
	cli := &http.Client{Transport: &http.Transport{}}
	defer cli.CloseIdleConnections()
	codigos = map[int]int{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() {
			resp, err := cli.Get(url)
			if err != nil {
				t.Error(err)
				return
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			mu.Lock()
			codigos[resp.StatusCode]++
			mu.Unlock()
		})
	}
	wg.Wait()
	return
}

func TestLimitarNuncaSuperaElMaximoYRechazaElResto(t *testing.T) {
	var activas, maxActivas atomic.Int32
	lento := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := activas.Add(1)
		defer activas.Add(-1)
		for {
			m := maxActivas.Load()
			if n <= m || maxActivas.CompareAndSwap(m, n) {
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	})
	srv := httptest.NewServer(Limitar(4, lento))
	defer srv.Close()
	c := lanzar(t, srv.URL, 20)
	if maxActivas.Load() > 4 {
		t.Fatalf("hubo %d solicitudes simultáneas, límite 4", maxActivas.Load())
	}
	if c[200] < 4 || c[200]+c[503] != 20 || c[503] == 0 {
		t.Fatalf("códigos: %v (esperado ≥4 × 200 y el resto 503)", c)
	}
}

func TestRetryAfterEnElRechazo(t *testing.T) {
	bloq := make(chan struct{})
	h := Limitar(1, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-bloq }))
	srv := httptest.NewServer(h)
	defer srv.Close()
	go http.Get(srv.URL) // ocupa la única plaza
	time.Sleep(50 * time.Millisecond)
	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	close(bloq)
	if resp.StatusCode != 503 || resp.Header.Get("Retry-After") == "" {
		t.Fatalf("código %d, Retry-After=%q", resp.StatusCode, resp.Header.Get("Retry-After"))
	}
}

func TestColaAbsorbeRafagaYAcotaLaEspera(t *testing.T) {
	var ejecutadas atomic.Int32
	h := LimitarConCola(2, 3, 500*time.Millisecond, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ejecutadas.Add(1)
		time.Sleep(50 * time.Millisecond)
	}))
	srv := httptest.NewServer(h)
	defer srv.Close()
	c := lanzar(t, srv.URL, 12) // 2 ejecutando + 3 en cola = 5 admitidas
	if c[200] < 5 || c[200]+c[503] != 12 || c[503] == 0 {
		t.Fatalf("códigos: %v", c)
	}
}

func TestLaCancelacionDelClienteLlegaAlHandler(t *testing.T) {
	canceladas := make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			close(canceladas)
		case <-time.After(5 * time.Second):
		}
	})
	srv := httptest.NewServer(h)
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL, nil)
	if _, err := http.DefaultClient.Do(req); err == nil {
		t.Fatal("se esperaba un error por el plazo del cliente")
	}
	select {
	case <-canceladas:
	case <-time.After(2 * time.Second):
		t.Fatal("el handler no se enteró de que el cliente se fue")
	}
	http.DefaultClient.CloseIdleConnections()
}

func TestApagadoOrdenadoEsperaALasSolicitudesEnCurso(t *testing.T) {
	enCurso := make(chan struct{})
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(enCurso)
		time.Sleep(300 * time.Millisecond)
		w.Write([]byte("terminé"))
	})
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	ctx, parar := context.WithCancel(context.Background())
	hecho := make(chan error, 1)
	go func() { hecho <- Servir(ctx, NuevoServidor(h), ln, 5*time.Second) }()

	respuesta := make(chan string, 1)
	go func() {
		resp, err := http.Get("http://" + ln.Addr().String())
		if err != nil {
			respuesta <- "error: " + err.Error()
			return
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		respuesta <- string(b)
	}()
	<-enCurso
	parar() // apagado con una solicitud a medias
	if got := <-respuesta; got != "terminé" {
		t.Fatalf("la solicitud en curso debía completarse, recibió %q", got)
	}
	if err := <-hecho; err != nil {
		t.Fatalf("Servir = %v", err)
	}
	if _, err := http.Get("http://" + ln.Addr().String()); err == nil {
		t.Fatal("tras el apagado no debe aceptar conexiones")
	}
	http.DefaultClient.CloseIdleConnections()
}

// Un cliente que envía las cabeceras a goteo (slowloris) no retiene la conexión para siempre.
func TestReadHeaderTimeoutCortaClientesLentos(t *testing.T) {
	srv := NuevoServidor(http.NotFoundHandler())
	srv.ReadHeaderTimeout = 150 * time.Millisecond
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	go srv.Serve(ln)
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	conn.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\n")) // cabeceras sin terminar
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	t0 := time.Now()
	_, err = bufio.NewReader(conn).ReadByte() // se cierra (o responde 408) al vencer el plazo
	if ne, ok := err.(net.Error); ok && ne.Timeout() {
		t.Fatal("el servidor no cortó la conexión lenta")
	}
	if d := time.Since(t0); d > time.Second {
		t.Fatalf("tardó %v en cortar", d)
	}
}
