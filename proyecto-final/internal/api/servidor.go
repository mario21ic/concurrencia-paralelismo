// Package api expone el planificador por HTTP/JSON (servidor) y ofrece un cliente tipado.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/http/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/mario21ic/golang-concurrencia-paralelismo/proyecto-final/internal/modelo"
	"github.com/mario21ic/golang-concurrencia-paralelismo/proyecto-final/internal/planificador"
)

const maxEsperaLarga = 30 * time.Second // tope del long polling (lease y espera de un trabajo)

// Servidor traduce HTTP a llamadas al planificador.
type Servidor struct {
	P   *planificador.Planificador
	Log *slog.Logger
}

// Handler devuelve el enrutador completo (API + métricas + pprof).
func (s *Servidor) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /jobs", s.enviar)
	mux.HandleFunc("GET /jobs/{id}", s.obtener)
	mux.HandleFunc("DELETE /jobs/{id}", s.cancelar)
	mux.HandleFunc("POST /lease", s.arrendar)
	mux.HandleFunc("POST /jobs/{id}/ack", s.ack)
	mux.HandleFunc("POST /jobs/{id}/nack", s.nack)
	mux.HandleFunc("POST /jobs/{id}/heartbeat", s.latido)
	mux.HandleFunc("GET /stats", s.stats)
	mux.HandleFunc("GET /metrics", s.metricas)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })

	// Perfilado en vivo: go tool pprof http://localhost:8080/debug/pprof/profile?seconds=10
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return s.registro(mux)
}

// registro anota método, ruta, código y duración de cada solicitud (excepto sondeos largos y salud).
func (s *Servidor) registro(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t0 := time.Now()
		rec := &codigoRec{ResponseWriter: w, codigo: 200}
		next.ServeHTTP(rec, r)
		if s.Log != nil && r.URL.Path != "/healthz" && r.URL.Path != "/lease" && rec.codigo >= 400 {
			s.Log.Warn("solicitud", "método", r.Method, "ruta", r.URL.Path, "código", rec.codigo, "dur", time.Since(t0).Round(time.Microsecond))
		}
	})
}

type codigoRec struct {
	http.ResponseWriter
	codigo int
}

func (c *codigoRec) WriteHeader(n int)           { c.codigo = n; c.ResponseWriter.WriteHeader(n) }
func (c *codigoRec) Unwrap() http.ResponseWriter { return c.ResponseWriter } // para http.ResponseController

type errorJSON struct {
	Error        string `json:"error"`
	Codigo       string `json:"codigo"`
	RetryAfterMS int64  `json:"retry_after_ms,omitempty"`
}

func escribirJSON(w http.ResponseWriter, codigo int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(codigo)
	json.NewEncoder(w).Encode(v)
}

// escribirError traduce errores del dominio a códigos HTTP. Es el único sitio que conoce la correspondencia.
func escribirError(w http.ResponseWriter, err error) {
	var lim *modelo.ErrLimite
	switch {
	case errors.As(err, &lim):
		w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(lim.RetryAfter.Seconds()))))
		escribirJSON(w, http.StatusTooManyRequests, errorJSON{err.Error(), "limite_tasa", lim.RetryAfter.Milliseconds()})
	case errors.Is(err, modelo.ErrColaLlena):
		w.Header().Set("Retry-After", "1")
		escribirJSON(w, http.StatusServiceUnavailable, errorJSON{err.Error(), "cola_llena", 1000})
	case errors.Is(err, modelo.ErrSolicitud):
		escribirJSON(w, http.StatusBadRequest, errorJSON{Error: err.Error(), Codigo: "solicitud"})
	case errors.Is(err, modelo.ErrNoEncontrado):
		escribirJSON(w, http.StatusNotFound, errorJSON{Error: err.Error(), Codigo: "no_encontrado"})
	case errors.Is(err, modelo.ErrArriendoPerdido):
		escribirJSON(w, http.StatusGone, errorJSON{Error: err.Error(), Codigo: "arriendo_perdido"})
	case errors.Is(err, modelo.ErrEstadoInvalido):
		escribirJSON(w, http.StatusConflict, errorJSON{Error: err.Error(), Codigo: "estado_invalido"})
	default:
		escribirJSON(w, http.StatusInternalServerError, errorJSON{Error: err.Error(), Codigo: "interno"})
	}
}

func leerJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		escribirError(w, fmt.Errorf("%w: %v", modelo.ErrSolicitud, err))
		return false
	}
	return true
}

func (s *Servidor) enviar(w http.ResponseWriter, r *http.Request) {
	var sol modelo.Solicitud
	if !leerJSON(w, r, &sol) {
		return
	}
	job, creado, err := s.P.Enviar(sol)
	if err != nil {
		escribirError(w, err)
		return
	}
	codigo := http.StatusCreated
	if !creado {
		codigo = http.StatusOK // reenvío idempotente
	}
	escribirJSON(w, codigo, job)
}

func (s *Servidor) obtener(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if v := r.URL.Query().Get("esperar"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			escribirError(w, fmt.Errorf("%w: esperar: %v", modelo.ErrSolicitud, err))
			return
		}
		d = min(d, maxEsperaLarga)
		http.NewResponseController(w).SetWriteDeadline(time.Now().Add(d + 5*time.Second))
		job, err := s.P.Esperar(r.Context(), id, d)
		if err != nil {
			escribirError(w, err)
			return
		}
		escribirJSON(w, http.StatusOK, job)
		return
	}
	job, ok := s.P.Obtener(id)
	if !ok {
		escribirError(w, modelo.ErrNoEncontrado)
		return
	}
	escribirJSON(w, http.StatusOK, job)
}

func (s *Servidor) cancelar(w http.ResponseWriter, r *http.Request) {
	if err := s.P.Cancelar(r.PathValue("id")); err != nil {
		escribirError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// PeticionLease es el cuerpo de POST /lease.
type PeticionLease struct {
	Worker   string `json:"worker"`
	Max      int    `json:"max"`
	EsperaMS int    `json:"espera_ms"`
}

func (s *Servidor) arrendar(w http.ResponseWriter, r *http.Request) {
	var req PeticionLease
	if !leerJSON(w, r, &req) {
		return
	}
	if req.Worker == "" {
		escribirError(w, fmt.Errorf("%w: falta worker", modelo.ErrSolicitud))
		return
	}
	espera := min(time.Duration(req.EsperaMS)*time.Millisecond, maxEsperaLarga)
	// El servidor tiene WriteTimeout=0 y cada sondeo largo fija su propio plazo de escritura.
	http.NewResponseController(w).SetWriteDeadline(time.Now().Add(espera + 5*time.Second))
	jobs, err := s.P.Arrendar(r.Context(), req.Worker, min(req.Max, 100), espera)
	if err != nil { // el cliente se fue: nada que responder
		return
	}
	escribirJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}

type cuerpoAck struct {
	Worker    string          `json:"worker"`
	Resultado json.RawMessage `json:"resultado,omitempty"`
}
type cuerpoNack struct {
	Worker       string `json:"worker"`
	Error        string `json:"error"`
	Reintentable bool   `json:"reintentable"`
}

func (s *Servidor) ack(w http.ResponseWriter, r *http.Request) {
	var c cuerpoAck
	if !leerJSON(w, r, &c) {
		return
	}
	if err := s.P.Ack(r.PathValue("id"), c.Worker, c.Resultado); err != nil {
		escribirError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Servidor) nack(w http.ResponseWriter, r *http.Request) {
	var c cuerpoNack
	if !leerJSON(w, r, &c) {
		return
	}
	if err := s.P.Nack(r.PathValue("id"), c.Worker, c.Error, c.Reintentable); err != nil {
		escribirError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Servidor) latido(w http.ResponseWriter, r *http.Request) {
	var c cuerpoAck
	if !leerJSON(w, r, &c) {
		return
	}
	if err := s.P.Latido(r.PathValue("id"), c.Worker); err != nil {
		escribirError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Servidor) stats(w http.ResponseWriter, _ *http.Request) {
	escribirJSON(w, http.StatusOK, s.P.Estadisticas())
}

// metricas escribe el formato de texto de Prometheus (sin depender de ninguna biblioteca).
func (s *Servidor) metricas(w http.ResponseWriter, _ *http.Request) {
	st := s.P.Estadisticas()
	m := &s.P.M
	var b strings.Builder
	cont := func(nombre, ayuda string, v int64) {
		fmt.Fprintf(&b, "# HELP jobq_%s %s\n# TYPE jobq_%s counter\njobq_%s %d\n", nombre, ayuda, nombre, nombre, v)
	}
	cont("enviados_total", "Trabajos admitidos.", m.Enviados.Load())
	cont("duplicados_total", "Envíos idempotentes repetidos.", m.Duplicados.Load())
	cont("rechazados_cola_llena_total", "Envíos rechazados por contrapresión.", m.RechazadosLlena.Load())
	cont("rechazados_tasa_total", "Envíos rechazados por la cuota del tenant.", m.RechazadosTasa.Load())
	cont("arrendados_total", "Arriendos concedidos.", m.Arrendados.Load())
	cont("completados_total", "Trabajos completados.", m.Completados.Load())
	cont("reintentos_total", "Trabajos enviados a reintento.", m.Reintentos.Load())
	cont("fallidos_total", "Trabajos en la cola de muertos.", m.Fallidos.Load())
	cont("arriendos_caducados_total", "Arriendos vencidos sin latido.", m.Caducados.Load())
	fmt.Fprintf(&b, "# HELP jobq_trabajos Trabajos por estado.\n# TYPE jobq_trabajos gauge\n")
	for _, e := range []modelo.Estado{modelo.Pendiente, modelo.Retardado, modelo.EnCurso, modelo.Completado, modelo.Fallido, modelo.Cancelado} {
		fmt.Fprintf(&b, "jobq_trabajos{estado=%q} %d\n", e, st.Estados[e])
	}
	fmt.Fprintf(&b, "# HELP jobq_pendientes_por_prioridad Cola por nivel.\n# TYPE jobq_pendientes_por_prioridad gauge\n")
	for n, v := range st.PorNivel {
		fmt.Fprintf(&b, "jobq_pendientes_por_prioridad{prioridad=\"%d\"} %d\n", n, v)
	}
	fmt.Fprintf(&b, "# TYPE jobq_workers_esperando gauge\njobq_workers_esperando %d\n", st.Esperando)
	hist := func(nombre string, h *planificador.Histograma) {
		fmt.Fprintf(&b, "# TYPE jobq_%s_segundos histogram\n", nombre)
		cotas, acum := h.Cubos()
		for i := range cotas {
			fmt.Fprintf(&b, "jobq_%s_segundos_bucket{le=\"%g\"} %d\n", nombre, cotas[i]/1000, acum[i])
		}
		fmt.Fprintf(&b, "jobq_%s_segundos_bucket{le=\"+Inf\"} %d\njobq_%s_segundos_sum %g\njobq_%s_segundos_count %d\n",
			nombre, h.Cuenta(), nombre, h.Media().Seconds()*float64(h.Cuenta()), nombre, h.Cuenta())
	}
	hist("espera", &m.Espera)
	hist("ejecucion", &m.Ejecucion)
	hist("total", &m.Total)
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.Write([]byte(b.String()))
}

// ---- Servidor HTTP con apagado ordenado (patrón Run(ctx) del módulo 15) ----

// NuevoHTTP devuelve un http.Server con tiempos límite. WriteTimeout es 0 porque los sondeos largos
// fijan su propio plazo con http.ResponseController; el resto de protecciones se mantienen.
func NuevoHTTP(h http.Handler) *http.Server {
	return &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}
}

// Servir atiende hasta que ctx se cancela; entonces deja de aceptar y espera a las solicitudes en curso
// (hasta `gracia`).
func Servir(ctx context.Context, srv *http.Server, ln net.Listener, gracia time.Duration) error {
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	paro, cancel := context.WithTimeout(context.Background(), gracia)
	defer cancel()
	if err := srv.Shutdown(paro); err != nil {
		srv.Close()
		return err
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
