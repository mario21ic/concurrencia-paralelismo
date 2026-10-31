// Worker: arrienda trabajos al coordinador y los ejecuta.
//
//	go run ./cmd/worker -url http://localhost:8080
package main

import (
	"context"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/mario21ic/golang-concurrencia-paralelismo/proyecto-final/internal/api"
	"github.com/mario21ic/golang-concurrencia-paralelismo/proyecto-final/internal/worker"
)

func envOr(clave, defecto string) string {
	if v := os.Getenv(clave); v != "" {
		return v
	}
	return defecto
}

func main() {
	host, _ := os.Hostname()
	var (
		url    = flag.String("url", envOr("COORDINADOR_URL", "http://localhost:8080"), "URL del coordinador")
		id     = flag.String("id", envOr("WORKER_ID", host), "identificador del worker")
		slots  = flag.Int("slots", 0, "trabajos simultáneos (0 = GOMAXPROCS)")
		espera = flag.Duration("espera", 5*time.Second, "duración de cada sondeo largo")
		gracia = flag.Duration("gracia", 10*time.Second, "gracia en el apagado")
	)
	flag.Parse()
	log := slog.New(slog.NewTextHandler(os.Stderr, nil)).With("worker", *id)

	cfg := worker.Config{ID: *id, Slots: *slots, Espera: *espera, Gracia: *gracia, Log: log}
	w := worker.New(api.NuevoCliente(*url), cfg, worker.Predeterminados())
	// En un contenedor con límite de CPU, Go 1.25+ ajusta GOMAXPROCS al límite (cgroups): lo registramos.
	log.Info("worker listo", "coordinador", *url, "gomaxprocs", runtime.GOMAXPROCS(0), "numcpu", runtime.NumCPU(), "slots", max(*slots, 0))

	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()
	w.Run(ctx)
	log.Info("worker detenido")
}
