package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/mario21ic/golang-concurrencia-paralelismo/proyecto-final/internal/modelo"
	"github.com/mario21ic/golang-concurrencia-paralelismo/proyecto-final/internal/planificador"
)

// Cliente habla con el coordinador. Es seguro para uso concurrente. Los errores del dominio se
// reconstruyen (errors.Is/As funcionan igual que con el planificador local).
type Cliente struct {
	Base string
	HTTP *http.Client
}

func NuevoCliente(base string) *Cliente {
	return &Cliente{Base: base, HTTP: &http.Client{Transport: &http.Transport{
		MaxIdleConns: 256, MaxIdleConnsPerHost: 256, IdleConnTimeout: 90 * time.Second,
	}}}
}

func (c *Cliente) hacer(ctx context.Context, metodo, ruta string, cuerpo, destino any) (int, error) {
	var rd io.Reader
	if cuerpo != nil {
		b, _ := json.Marshal(cuerpo)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, metodo, c.Base+ruta, rd)
	if err != nil {
		return 0, err
	}
	if cuerpo != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		var e errorJSON
		json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&e)
		return resp.StatusCode, reconstruir(resp.StatusCode, e)
	}
	if destino != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(destino); err != nil {
			return resp.StatusCode, err
		}
	}
	return resp.StatusCode, nil
}

func reconstruir(codigo int, e errorJSON) error {
	envolver := func(base error) error { return fmt.Errorf("%w: %s", base, e.Error) }
	switch e.Codigo {
	case "limite_tasa":
		return &modelo.ErrLimite{RetryAfter: time.Duration(e.RetryAfterMS) * time.Millisecond}
	case "cola_llena":
		return envolver(modelo.ErrColaLlena)
	case "solicitud":
		return envolver(modelo.ErrSolicitud)
	case "no_encontrado":
		return envolver(modelo.ErrNoEncontrado)
	case "arriendo_perdido":
		return envolver(modelo.ErrArriendoPerdido)
	case "estado_invalido":
		return envolver(modelo.ErrEstadoInvalido)
	}
	return fmt.Errorf("coordinador respondió %d: %s", codigo, e.Error)
}

func (c *Cliente) Enviar(ctx context.Context, s modelo.Solicitud) (job modelo.Job, creado bool, err error) {
	codigo, err := c.hacer(ctx, "POST", "/jobs", s, &job)
	return job, codigo == http.StatusCreated, err
}

func (c *Cliente) Obtener(ctx context.Context, id string) (job modelo.Job, err error) {
	_, err = c.hacer(ctx, "GET", "/jobs/"+url.PathEscape(id), nil, &job)
	return
}

// Esperar hace un sondeo largo: vuelve cuando el trabajo termina o tras `max` (con el estado actual).
func (c *Cliente) Esperar(ctx context.Context, id string, max time.Duration) (job modelo.Job, err error) {
	_, err = c.hacer(ctx, "GET", fmt.Sprintf("/jobs/%s?esperar=%s", url.PathEscape(id), max), nil, &job)
	return
}

func (c *Cliente) Cancelar(ctx context.Context, id string) error {
	_, err := c.hacer(ctx, "DELETE", "/jobs/"+url.PathEscape(id), nil, nil)
	return err
}

func (c *Cliente) Arrendar(ctx context.Context, worker string, max int, espera time.Duration) ([]modelo.Job, error) {
	var r struct {
		Jobs []modelo.Job `json:"jobs"`
	}
	_, err := c.hacer(ctx, "POST", "/lease", PeticionLease{Worker: worker, Max: max, EsperaMS: int(espera / time.Millisecond)}, &r)
	return r.Jobs, err
}

func (c *Cliente) Ack(ctx context.Context, id, worker string, resultado json.RawMessage) error {
	_, err := c.hacer(ctx, "POST", "/jobs/"+url.PathEscape(id)+"/ack", cuerpoAck{worker, resultado}, nil)
	return err
}

func (c *Cliente) Nack(ctx context.Context, id, worker, motivo string, reintentable bool) error {
	_, err := c.hacer(ctx, "POST", "/jobs/"+url.PathEscape(id)+"/nack", cuerpoNack{worker, motivo, reintentable}, nil)
	return err
}

func (c *Cliente) Latido(ctx context.Context, id, worker string) error {
	_, err := c.hacer(ctx, "POST", "/jobs/"+url.PathEscape(id)+"/heartbeat", cuerpoAck{Worker: worker}, nil)
	return err
}

func (c *Cliente) Estadisticas(ctx context.Context) (st planificador.Estadisticas, err error) {
	_, err = c.hacer(ctx, "GET", "/stats", nil, &st)
	return
}

// Pausa devuelve cuánto esperar antes de reintentar un envío rechazado por contrapresión o cuota.
func Pausa(err error) (time.Duration, bool) {
	var lim *modelo.ErrLimite
	switch {
	case errors.As(err, &lim):
		return lim.RetryAfter, true
	case errors.Is(err, modelo.ErrColaLlena):
		return 200 * time.Millisecond, true
	}
	return 0, false
}
