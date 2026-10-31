// Coordinador: cola de trabajos con prioridades, reintentos, rate limiting y métricas.
//
//	go run ./cmd/coordinador -addr :8080
package main

import (
	"context"
	"flag"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"github.com/mario21ic/golang-concurrencia-paralelismo/proyecto-final/internal/api"
	"github.com/mario21ic/golang-concurrencia-paralelismo/proyecto-final/internal/planificador"
)

// envOr permite configurar con variables de entorno (útil en contenedores) y con flags.
func envOr(clave, defecto string) string {
	if v := os.Getenv(clave); v != "" {
		return v
	}
	return defecto
}

func envDur(clave string, defecto time.Duration) time.Duration {
	if v, err := time.ParseDuration(os.Getenv(clave)); err == nil {
		return v
	}
	return defecto
}

func envInt(clave string, defecto int) int {
	if v, err := strconv.Atoi(os.Getenv(clave)); err == nil {
		return v
	}
	return defecto
}

func main() {
	var (
		addr      = flag.String("addr", envOr("ADDR", ":8080"), "dirección de escucha")
		capacidad = flag.Int("capacidad", envInt("CAPACIDAD", 100_000), "máximo de trabajos en espera (contrapresión)")
		env       = flag.Duration("envejecimiento", envDur("ENVEJECIMIENTO", 5*time.Second), "espera que equivale a subir un nivel de prioridad (0 = estricta)")
		ttl       = flag.Duration("arriendo-ttl", envDur("ARRIENDO_TTL", 10*time.Second), "vigencia de un arriendo sin latidos")
		tasa      = flag.Float64("tasa-tenant", float64(envInt("TASA_TENANT", 0)), "envíos/s por tenant (0 = sin límite)")
		rafaga    = flag.Int("rafaga-tenant", envInt("RAFAGA_TENANT", 100), "ráfaga por tenant")
		base      = flag.Duration("backoff-base", envDur("BACKOFF_BASE", 200*time.Millisecond), "backoff inicial de los reintentos")
		maxBack   = flag.Duration("backoff-max", envDur("BACKOFF_MAX", 30*time.Second), "tope del backoff")
		intentos  = flag.Int("max-intentos", envInt("MAX_INTENTOS", 3), "intentos por defecto")
		retencion = flag.Duration("retencion", envDur("RETENCION", 10*time.Minute), "cuánto se guardan los trabajos terminados")
		gracia    = flag.Duration("gracia", envDur("GRACIA", 15*time.Second), "gracia en el apagado")
		logCada   = flag.Duration("log-cada", envDur("LOG_CADA", 10*time.Second), "cada cuánto registrar estadísticas (0 = nunca)")
	)
	flag.Parse()
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))

	p := planificador.Nuevo(planificador.Config{
		Capacidad: *capacidad, Envejecimiento: *env, ArriendoTTL: *ttl, TasaTenant: rate.Limit(*tasa),
		RafagaTenant: *rafaga, BackoffBase: *base, BackoffMax: *maxBack, MaxIntentos: *intentos, Retencion: *retencion,
	})
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Error("no se puede escuchar", "error", err)
		os.Exit(1)
	}
	log.Info("coordinador listo", "addr", ln.Addr().String(), "capacidad", *capacidad, "envejecimiento", *env,
		"arriendo_ttl", *ttl, "tasa_tenant", *tasa, "gomaxprocs", os.Getenv("GOMAXPROCS"))

	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()

	// Ciclo de vida atado a ctx: si una parte falla o llega SIGTERM, todas se paran y se espera a todas.
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		if err := p.Run(gctx); err != context.Canceled {
			return err
		}
		return nil
	})
	g.Go(func() error {
		srv := api.NuevoHTTP((&api.Servidor{P: p, Log: log}).Handler())
		return api.Servir(gctx, srv, ln, *gracia)
	})
	if *logCada > 0 {
		g.Go(func() error {
			t := time.NewTicker(*logCada)
			defer t.Stop()
			for {
				select {
				case <-t.C:
					st := p.Estadisticas()
					log.Info("estado", "en_espera", st.EnEspera, "en_curso", st.Estados["en_curso"],
						"completados", st.Totales["completados"], "fallidos", st.Totales["fallidos"],
						"reintentos", st.Totales["reintentos"], "workers_esperando", st.Esperando)
				case <-gctx.Done():
					return nil
				}
			}
		})
	}
	if err := g.Wait(); err != nil {
		log.Error("terminó con error", "error", err)
		os.Exit(1)
	}
	log.Info("apagado completo")
}
