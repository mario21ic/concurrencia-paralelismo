package worker_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/goleak"

	"github.com/mario21ic/golang-concurrencia-paralelismo/proyecto-final/internal/api"
	"github.com/mario21ic/golang-concurrencia-paralelismo/proyecto-final/internal/modelo"
	"github.com/mario21ic/golang-concurrencia-paralelismo/proyecto-final/internal/planificador"
	"github.com/mario21ic/golang-concurrencia-paralelismo/proyecto-final/internal/worker"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

func rapido(c planificador.Config) planificador.Config {
	if c.BackoffBase == 0 {
		c.BackoffBase = 10 * time.Millisecond
	}
	return c
}

func arrancar(t *testing.T, cfg planificador.Config) (*planificador.Planificador, *api.Cliente) {
	t.Helper()
	p := planificador.Nuevo(rapido(cfg))
	ctx, cancel := context.WithCancel(context.Background())
	hecho := make(chan struct{})
	go func() { p.Run(ctx); close(hecho) }()
	srv := newServer(p)
	cli := api.NuevoCliente(srv.URL)
	t.Cleanup(func() { cli.HTTP.CloseIdleConnections(); srv.Close(); cancel(); <-hecho })
	return p, cli
}

// lanzar ejecuta un worker y devuelve una función que lo detiene y espera a que termine.
func lanzar(t *testing.T, cli *api.Cliente, cfg worker.Config, h map[string]worker.Handler) (detener func()) {
	t.Helper()
	if cfg.ID == "" {
		cfg.ID = "w1"
	}
	if cfg.Espera == 0 {
		cfg.Espera = 100 * time.Millisecond
	}
	ctx, cancel := context.WithCancel(context.Background())
	hecho := make(chan struct{})
	w := worker.New(cli, cfg, h)
	go func() { w.Run(ctx); close(hecho) }()
	var una atomic.Bool
	detener = func() {
		if una.CompareAndSwap(false, true) {
			cancel()
			<-hecho
		}
	}
	t.Cleanup(detener)
	return
}

func esperarTodos(t *testing.T, cli *api.Cliente, ids []string) []modelo.Job {
	t.Helper()
	var out []modelo.Job
	for _, id := range ids {
		for {
			j, err := cli.Esperar(t.Context(), id, 2*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			if j.Estado.Terminal() {
				out = append(out, j)
				break
			}
			if t.Context().Err() != nil {
				t.Fatal("test cancelado")
			}
		}
	}
	return out
}

func enviar(t *testing.T, cli *api.Cliente, s modelo.Solicitud) string {
	t.Helper()
	j, _, err := cli.Enviar(t.Context(), s)
	if err != nil {
		t.Fatal(err)
	}
	return j.ID
}

func TestWorkerEjecutaTodosLosTrabajos(t *testing.T) {
	_, cli := arrancar(t, planificador.Config{})
	lanzar(t, cli, worker.Config{Slots: 4}, worker.Predeterminados())
	var ids []string
	for i := range 50 {
		ids = append(ids, enviar(t, cli, modelo.Solicitud{Tipo: "eco", Payload: json.RawMessage(`{"i":` + string(rune('0'+i%10)) + `}`)}))
	}
	for _, j := range esperarTodos(t, cli, ids) {
		if j.Estado != modelo.Completado || len(j.Resultado) == 0 {
			t.Fatalf("%+v", j)
		}
	}
}

func TestNuncaMasDeSlotsTrabajosALaVez(t *testing.T) {
	_, cli := arrancar(t, planificador.Config{})
	var activos, maximo atomic.Int32
	h := map[string]worker.Handler{"medir": func(ctx context.Context, _ json.RawMessage) (json.RawMessage, error) {
		n := activos.Add(1)
		defer activos.Add(-1)
		for {
			m := maximo.Load()
			if n <= m || maximo.CompareAndSwap(m, n) {
				break
			}
		}
		time.Sleep(40 * time.Millisecond)
		return nil, nil
	}}
	lanzar(t, cli, worker.Config{Slots: 3}, h)
	var ids []string
	for range 15 {
		ids = append(ids, enviar(t, cli, modelo.Solicitud{Tipo: "medir"}))
	}
	esperarTodos(t, cli, ids)
	if maximo.Load() != 3 {
		t.Fatalf("máximo simultáneo = %d, esperado exactamente 3", maximo.Load())
	}
}

func TestReintentosYColaDeMuertos(t *testing.T) {
	p, cli := arrancar(t, planificador.Config{})
	lanzar(t, cli, worker.Config{Slots: 2}, worker.Predeterminados())

	siempre := enviar(t, cli, modelo.Solicitud{Tipo: "fallo", Payload: json.RawMessage(`{"prob":1}`), MaxIntentos: 3})
	permanente := enviar(t, cli, modelo.Solicitud{Tipo: "fallo", Payload: json.RawMessage(`{"prob":1,"permanente":true}`), MaxIntentos: 3})
	desconocido := enviar(t, cli, modelo.Solicitud{Tipo: "no-existe"})
	res := esperarTodos(t, cli, []string{siempre, permanente, desconocido})

	if res[0].Estado != modelo.Fallido || res[0].Intento != 3 {
		t.Errorf("reintentable: %+v (esperado fallido tras 3 intentos)", res[0])
	}
	if res[1].Estado != modelo.Fallido || res[1].Intento != 1 {
		t.Errorf("permanente: %+v (esperado fallido al 1.er intento)", res[1])
	}
	if res[2].Estado != modelo.Fallido || res[2].Intento != 1 || !strings.Contains(res[2].Error, "desconocido") {
		t.Errorf("tipo desconocido: %+v", res[2])
	}
	if p.M.Reintentos.Load() != 2 {
		t.Errorf("reintentos = %d, esperado 2", p.M.Reintentos.Load())
	}
}

func TestUnPanicoNoMataAlWorker(t *testing.T) {
	_, cli := arrancar(t, planificador.Config{})
	lanzar(t, cli, worker.Config{Slots: 2}, worker.Predeterminados())
	malo := enviar(t, cli, modelo.Solicitud{Tipo: "panico", MaxIntentos: 1})
	bueno := enviar(t, cli, modelo.Solicitud{Tipo: "eco"})
	res := esperarTodos(t, cli, []string{malo, bueno})
	if res[0].Estado != modelo.Fallido || !strings.Contains(res[0].Error, "pánico") {
		t.Fatalf("pánico: %+v", res[0])
	}
	if res[1].Estado != modelo.Completado {
		t.Fatalf("el worker debía seguir sirviendo: %+v", res[1])
	}
}

func TestTiempoLimitePorTrabajo(t *testing.T) {
	_, cli := arrancar(t, planificador.Config{})
	lanzar(t, cli, worker.Config{}, worker.Predeterminados())
	id := enviar(t, cli, modelo.Solicitud{Tipo: "dormir", Payload: json.RawMessage(`{"ms":5000}`), TimeoutMS: 50, MaxIntentos: 1})
	t0 := time.Now()
	j := esperarTodos(t, cli, []string{id})[0]
	if j.Estado != modelo.Fallido || !strings.Contains(j.Error, "deadline") || time.Since(t0) > 2*time.Second {
		t.Fatalf("%+v tras %v", j, time.Since(t0))
	}
}

func TestLosLatidosEvitanQueCaduqueUnTrabajoLargo(t *testing.T) {
	p, cli := arrancar(t, planificador.Config{ArriendoTTL: 150 * time.Millisecond})
	lanzar(t, cli, worker.Config{}, worker.Predeterminados())
	id := enviar(t, cli, modelo.Solicitud{Tipo: "dormir", Payload: json.RawMessage(`{"ms":700}`)}) // > 4 × TTL
	j := esperarTodos(t, cli, []string{id})[0]
	if j.Estado != modelo.Completado || j.Intento != 1 || p.M.Caducados.Load() != 0 {
		t.Fatalf("%+v caducados=%d", j, p.M.Caducados.Load())
	}
}

func TestSinLatidosElTrabajoPasaAOtroWorker(t *testing.T) {
	// Un worker "colgado": arrienda y no hace nada. Otro worker real acaba el trabajo cuando caduca.
	p, cli := arrancar(t, planificador.Config{ArriendoTTL: 200 * time.Millisecond})
	id := enviar(t, cli, modelo.Solicitud{Tipo: "eco"})
	jobs, _ := cli.Arrendar(t.Context(), "colgado", 1, time.Second)
	if len(jobs) != 1 {
		t.Fatal("no se arrendó")
	}
	lanzar(t, cli, worker.Config{ID: "sano"}, worker.Predeterminados())
	j := esperarTodos(t, cli, []string{id})[0]
	if j.Estado != modelo.Completado || j.Intento != 2 || p.M.Caducados.Load() != 1 {
		t.Fatalf("%+v caducados=%d", j, p.M.Caducados.Load())
	}
}

func TestApagadoConGraciaTerminaLosTrabajosEnCurso(t *testing.T) {
	_, cli := arrancar(t, planificador.Config{})
	detener := lanzar(t, cli, worker.Config{Gracia: 3 * time.Second}, worker.Predeterminados())
	id := enviar(t, cli, modelo.Solicitud{Tipo: "dormir", Payload: json.RawMessage(`{"ms":300}`)})
	esperarEnCurso(t, cli, id)
	t0 := time.Now()
	detener() // ctx cancelado con el trabajo a medias
	j, _ := cli.Obtener(t.Context(), id)
	if j.Estado != modelo.Completado || time.Since(t0) < 100*time.Millisecond {
		t.Fatalf("%+v tras %v: con gracia el trabajo debía completarse", j, time.Since(t0))
	}
}

func TestApagadoSinGraciaDevuelveElTrabajoALaCola(t *testing.T) {
	_, cli := arrancar(t, planificador.Config{BackoffBase: time.Hour})
	detener := lanzar(t, cli, worker.Config{Gracia: 30 * time.Millisecond}, worker.Predeterminados())
	id := enviar(t, cli, modelo.Solicitud{Tipo: "dormir", Payload: json.RawMessage(`{"ms":5000}`)})
	esperarEnCurso(t, cli, id)
	detener()
	j, _ := cli.Obtener(t.Context(), id)
	if j.Estado != modelo.Retardado || !strings.Contains(j.Error, "apagándose") {
		t.Fatalf("%+v: debía volver a la cola (retardado) para que otro worker lo recoja", j)
	}
}

func esperarEnCurso(t *testing.T, cli *api.Cliente, id string) {
	t.Helper()
	for range 200 {
		if j, _ := cli.Obtener(t.Context(), id); j.Estado == modelo.EnCurso {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("el trabajo nunca pasó a en_curso")
}

func TestHashEsDeterminista(t *testing.T) {
	h := worker.Predeterminados()["hash"]
	a, _ := h(t.Context(), json.RawMessage(`{"rondas":1000,"semilla":"x"}`))
	b, _ := h(t.Context(), json.RawMessage(`{"rondas":1000,"semilla":"x"}`))
	c, _ := h(t.Context(), json.RawMessage(`{"rondas":1001,"semilla":"x"}`))
	if string(a) != string(b) || string(a) == string(c) {
		t.Fatalf("%s %s %s", a, b, c)
	}
}
