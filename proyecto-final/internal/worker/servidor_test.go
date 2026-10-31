package worker_test

import (
	"net/http/httptest"

	"github.com/mario21ic/golang-concurrencia-paralelismo/proyecto-final/internal/api"
	"github.com/mario21ic/golang-concurrencia-paralelismo/proyecto-final/internal/planificador"
)

func newServer(p *planificador.Planificador) *httptest.Server {
	return httptest.NewServer((&api.Servidor{P: p}).Handler())
}
