package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/mario21ic/golang-concurrencia-paralelismo/proyecto-final/internal/api"
	"github.com/mario21ic/golang-concurrencia-paralelismo/proyecto-final/internal/modelo"
	"github.com/mario21ic/golang-concurrencia-paralelismo/proyecto-final/internal/planificador"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

// arrancar levanta un coordinador real en un servidor de pruebas y devuelve su cliente.
func arrancar(t *testing.T, cfg planificador.Config) (*planificador.Planificador, *api.Cliente, string) {
	t.Helper()
	p := planificador.Nuevo(cfg)
	ctx, cancel := context.WithCancel(context.Background())
	hecho := make(chan struct{})
	go func() { p.Run(ctx); close(hecho) }()
	srv := httptest.NewServer((&api.Servidor{P: p}).Handler())
	cli := api.NuevoCliente(srv.URL)
	t.Cleanup(func() {
		cli.HTTP.CloseIdleConnections()
		srv.Close()
		cancel()
		<-hecho
	})
	return p, cli, srv.URL
}

func TestEnviarObtenerYCicloCompleto(t *testing.T) {
	_, cli, _ := arrancar(t, planificador.Config{})
	ctx := t.Context()

	j, creado, err := cli.Enviar(ctx, modelo.Solicitud{Tipo: "eco", Payload: json.RawMessage(`{"a":1}`), Prioridad: 2})
	if err != nil || !creado || j.Estado != modelo.Pendiente || j.Prioridad != 2 {
		t.Fatalf("Enviar: %+v creado=%v err=%v", j, creado, err)
	}
	jobs, err := cli.Arrendar(ctx, "w1", 5, time.Second)
	if err != nil || len(jobs) != 1 || jobs[0].ID != j.ID || jobs[0].ArriendoMS == 0 {
		t.Fatalf("Arrendar: %+v err=%v", jobs, err)
	}
	if err := cli.Latido(ctx, j.ID, "w1"); err != nil {
		t.Fatalf("Latido: %v", err)
	}
	if err := cli.Ack(ctx, j.ID, "w1", json.RawMessage(`"hecho"`)); err != nil {
		t.Fatalf("Ack: %v", err)
	}
	fin, err := cli.Esperar(ctx, j.ID, time.Second)
	if err != nil || fin.Estado != modelo.Completado || string(fin.Resultado) != `"hecho"` {
		t.Fatalf("Esperar: %+v err=%v", fin, err)
	}
}

func TestErroresSeTraducenAHTTPYSeReconstruyen(t *testing.T) {
	p, cli, _ := arrancar(t, planificador.Config{Capacidad: 1, TasaTenant: 1, RafagaTenant: 100})
	ctx := t.Context()

	if _, _, err := cli.Enviar(ctx, modelo.Solicitud{Tipo: ""}); !errors.Is(err, modelo.ErrSolicitud) {
		t.Fatalf("solicitud inválida: %v", err)
	}
	if _, err := cli.Obtener(ctx, "nope"); !errors.Is(err, modelo.ErrNoEncontrado) {
		t.Fatalf("no encontrado: %v", err)
	}
	a, _, _ := cli.Enviar(ctx, modelo.Solicitud{Tipo: "x"})
	_, _, err := cli.Enviar(ctx, modelo.Solicitud{Tipo: "y"}) // capacidad 1
	if !errors.Is(err, modelo.ErrColaLlena) {
		t.Fatalf("cola llena: %v", err)
	}
	if err := cli.Ack(ctx, a.ID, "w", nil); !errors.Is(err, modelo.ErrArriendoPerdido) {
		t.Fatalf("ack sin arriendo: %v", err)
	}
	if err := cli.Cancelar(ctx, a.ID); err != nil {
		t.Fatalf("cancelar: %v", err)
	}
	if err := cli.Cancelar(ctx, a.ID); !errors.Is(err, modelo.ErrEstadoInvalido) {
		t.Fatalf("cancelar dos veces: %v", err)
	}
	if p.M.RechazadosLlena.Load() != 1 {
		t.Fatalf("rechazados = %d", p.M.RechazadosLlena.Load())
	}
}

func TestCodigosYCabecerasDeContrapresion(t *testing.T) {
	_, _, url := arrancar(t, planificador.Config{Capacidad: 1, TasaTenant: 0.5, RafagaTenant: 1})
	post := func(tenant string) *http.Response {
		resp, err := http.Post(url+"/jobs", "application/json", strings.NewReader(`{"tipo":"x","tenant":"`+tenant+`"}`))
		if err != nil {
			t.Fatal(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		return resp
	}
	if r := post("a"); r.StatusCode != http.StatusCreated {
		t.Fatalf("1.º envío: %d", r.StatusCode)
	}
	if r := post("b"); r.StatusCode != http.StatusServiceUnavailable || r.Header.Get("Retry-After") == "" {
		t.Fatalf("cola llena: %d Retry-After=%q", r.StatusCode, r.Header.Get("Retry-After"))
	}
	// vaciamos la cola para llegar al límite de tasa del tenant "a"
	resp, _ := http.Post(url+"/lease", "application/json", strings.NewReader(`{"worker":"w","max":5}`))
	resp.Body.Close()
	if r := post("a"); r.StatusCode != http.StatusTooManyRequests || r.Header.Get("Retry-After") == "" {
		t.Fatalf("límite de tasa: %d Retry-After=%q", r.StatusCode, r.Header.Get("Retry-After"))
	}
}

func TestIdempotenciaPorHTTP(t *testing.T) {
	_, cli, _ := arrancar(t, planificador.Config{})
	s := modelo.Solicitud{Tipo: "x", Clave: "k1"}
	a, creadoA, _ := cli.Enviar(t.Context(), s)
	b, creadoB, _ := cli.Enviar(t.Context(), s)
	if !creadoA || creadoB || a.ID != b.ID {
		t.Fatalf("creados %v,%v ids %s,%s", creadoA, creadoB, a.ID, b.ID)
	}
}

func TestSondeoLargoDevuelveVacioAlVencer(t *testing.T) {
	_, cli, _ := arrancar(t, planificador.Config{})
	t0 := time.Now()
	jobs, err := cli.Arrendar(t.Context(), "w", 1, 150*time.Millisecond)
	if err != nil || len(jobs) != 0 || time.Since(t0) < 140*time.Millisecond {
		t.Fatalf("jobs=%d err=%v tras %v", len(jobs), err, time.Since(t0))
	}
}

func TestSondeoLargoSeDespiertaConUnEnvio(t *testing.T) {
	_, cli, _ := arrancar(t, planificador.Config{})
	res := make(chan int)
	go func() {
		jobs, _ := cli.Arrendar(t.Context(), "w", 1, 5*time.Second)
		res <- len(jobs)
	}()
	time.Sleep(100 * time.Millisecond)
	t0 := time.Now()
	cli.Enviar(t.Context(), modelo.Solicitud{Tipo: "x"})
	if n := <-res; n != 1 || time.Since(t0) > time.Second {
		t.Fatalf("jobs=%d tras %v", n, time.Since(t0))
	}
}

func TestEsperarNoBloqueaParaSiempre(t *testing.T) {
	_, cli, _ := arrancar(t, planificador.Config{})
	j, _, _ := cli.Enviar(t.Context(), modelo.Solicitud{Tipo: "x"})
	got, err := cli.Esperar(t.Context(), j.ID, 100*time.Millisecond)
	if err != nil || got.Estado != modelo.Pendiente {
		t.Fatalf("%+v %v", got, err)
	}
}

func TestMetricasEstadisticasYPprof(t *testing.T) {
	_, cli, url := arrancar(t, planificador.Config{})
	j, _, _ := cli.Enviar(t.Context(), modelo.Solicitud{Tipo: "x", Prioridad: 3})
	jobs, _ := cli.Arrendar(t.Context(), "w", 1, time.Second)
	cli.Ack(t.Context(), jobs[0].ID, "w", nil)
	_ = j

	get := func(ruta string) string {
		resp, err := http.Get(url + ruta)
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("GET %s: %v %v", ruta, resp, err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return string(b)
	}
	m := get("/metrics")
	for _, quiero := range []string{"jobq_enviados_total 1", "jobq_completados_total 1", `jobq_trabajos{estado="completado"} 1`, "jobq_total_segundos_count 1"} {
		if !strings.Contains(m, quiero) {
			t.Errorf("/metrics no contiene %q", quiero)
		}
	}
	st, err := cli.Estadisticas(t.Context())
	if err != nil || st.Totales["completados"] != 1 {
		t.Fatalf("stats: %+v %v", st, err)
	}
	if !strings.Contains(get("/debug/pprof/"), "goroutine") {
		t.Error("pprof no disponible")
	}
	if get("/healthz") != "ok" {
		t.Error("healthz")
	}
}

func TestApagadoOrdenadoEsperaAlSondeoEnCurso(t *testing.T) {
	p := planificador.Nuevo(planificador.Config{})
	pctx, pcancel := context.WithCancel(context.Background())
	go p.Run(pctx)
	defer pcancel()
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	ctx, parar := context.WithCancel(context.Background())
	hecho := make(chan error, 1)
	go func() { hecho <- api.Servir(ctx, api.NuevoHTTP((&api.Servidor{P: p}).Handler()), ln, 5*time.Second) }()

	cli := api.NuevoCliente("http://" + ln.Addr().String())
	defer cli.HTTP.CloseIdleConnections()
	res := make(chan error, 1)
	go func() {
		// un sondeo largo en curso cuando llega la orden de apagado: Shutdown espera a que termine
		_, err := cli.Arrendar(context.Background(), "w", 1, 300*time.Millisecond)
		res <- err
	}()
	time.Sleep(100 * time.Millisecond)
	parar()
	if err := <-res; err != nil {
		t.Fatalf("el sondeo en curso debía completarse: %v", err)
	}
	if err := <-hecho; err != nil {
		t.Fatalf("Servir = %v", err)
	}
}
