// Generador de carga: envía N trabajos, espera a que terminen y resume throughput y latencias.
//
//	go run ./cmd/loadgen -url http://localhost:8080 -n 2000 -tipo hash -payload '{"rondas":200000}'
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"os/signal"
	"slices"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/mario21ic/golang-concurrencia-paralelismo/proyecto-final/internal/api"
	"github.com/mario21ic/golang-concurrencia-paralelismo/proyecto-final/internal/modelo"
)

func envOr(clave, defecto string) string {
	if v := os.Getenv(clave); v != "" {
		return v
	}
	return defecto
}

func pct(d []time.Duration, q float64) time.Duration {
	if len(d) == 0 {
		return 0
	}
	return d[int(float64(len(d)-1)*q)]
}

func main() {
	var (
		url     = flag.String("url", envOr("COORDINADOR_URL", "http://localhost:8080"), "URL del coordinador")
		n       = flag.Int("n", 1000, "número de trabajos")
		tipo    = flag.String("tipo", "hash", "tipo de trabajo (hash, dormir, eco, fallo)")
		payload = flag.String("payload", `{"rondas":200000}`, "payload JSON")
		conc    = flag.Int("concurrencia", 64, "envíos simultáneos")
		mix     = flag.Bool("mix", false, "repartir prioridades 0-3 (10 % crítica, 20 % alta, 40 % media, 30 % baja)")
		prio    = flag.Int("prioridad", 1, "prioridad fija si no hay -mix")
		tenants = flag.Int("tenants", 1, "número de tenants entre los que repartir")
		salida  = flag.String("json", "", "escribir el resumen en este fichero JSON")
	)
	flag.Parse()
	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()
	cli := api.NuevoCliente(*url)

	// 1) Enviar, con reintento ante contrapresión (429/503), respetando Retry-After.
	t0 := time.Now()
	ids := make([]string, *n)
	var mu sync.Mutex
	var rechazos int
	g, gctx := errgroup.WithContext(ctx)
	g.SetLimit(*conc)
	for i := range *n {
		g.Go(func() error {
			p := *prio
			if *mix {
				switch r := rand.IntN(100); {
				case r < 10:
					p = 3
				case r < 30:
					p = 2
				case r < 70:
					p = 1
				default:
					p = 0
				}
			}
			sol := modelo.Solicitud{Tipo: *tipo, Payload: json.RawMessage(*payload), Prioridad: p, Tenant: fmt.Sprintf("t%d", i%*tenants)}
			for {
				j, _, err := cli.Enviar(gctx, sol)
				if err == nil {
					ids[i] = j.ID
					return nil
				}
				if d, ok := api.Pausa(err); ok {
					mu.Lock()
					rechazos++
					mu.Unlock()
					select {
					case <-time.After(d/2 + time.Duration(rand.Int64N(int64(d/2)+1))):
						continue
					case <-gctx.Done():
						return gctx.Err()
					}
				}
				return err
			}
		})
	}
	if err := g.Wait(); err != nil {
		fmt.Fprintln(os.Stderr, "error al enviar:", err)
		os.Exit(1)
	}
	envio := time.Since(t0)

	// 2) Esperar a que todos terminen (sondeo largo por trabajo).
	jobs := make([]modelo.Job, *n)
	g, gctx = errgroup.WithContext(ctx)
	g.SetLimit(*conc)
	for i, id := range ids {
		g.Go(func() error {
			for {
				j, err := cli.Esperar(gctx, id, 20*time.Second)
				if err != nil {
					return err
				}
				if j.Estado.Terminal() {
					jobs[i] = j
					return nil
				}
			}
		})
	}
	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, "error al esperar:", err)
		os.Exit(1)
	}
	total := time.Since(t0)

	// 3) Resumen.
	var ok, fallidos, reintentos int
	var lat []time.Duration
	porPrio := map[int][]time.Duration{}
	for _, j := range jobs {
		if j.ID == "" {
			continue
		}
		reintentos += j.Intento - 1
		if j.Estado == modelo.Completado {
			ok++
			d := j.Fin.Sub(j.Creado)
			lat = append(lat, d)
			porPrio[j.Prioridad] = append(porPrio[j.Prioridad], d)
		} else {
			fallidos++
		}
	}
	slices.Sort(lat)
	fmt.Printf("trabajos: %d (tipo %s) | completados %d | fallidos %d | reintentos %d | envíos rechazados y repetidos %d\n", *n, *tipo, ok, fallidos, reintentos, rechazos)
	fmt.Printf("envío: %v (%.0f jobs/s) | total: %v | throughput: %.0f jobs/s\n",
		envio.Round(time.Millisecond), float64(*n)/envio.Seconds(), total.Round(time.Millisecond), float64(ok)/total.Seconds())
	fmt.Printf("latencia de extremo a extremo (creado→fin): p50 %v | p95 %v | p99 %v | máx %v\n",
		pct(lat, .5).Round(time.Millisecond), pct(lat, .95).Round(time.Millisecond), pct(lat, .99).Round(time.Millisecond), pct(lat, 1).Round(time.Millisecond))
	if *mix {
		for p := modelo.Niveles - 1; p >= 0; p-- {
			d := porPrio[p]
			slices.Sort(d)
			fmt.Printf("  prioridad %d: %5d trabajos | p50 %-8v p95 %-8v máx %v\n", p, len(d), pct(d, .5).Round(time.Millisecond), pct(d, .95).Round(time.Millisecond), pct(d, 1).Round(time.Millisecond))
		}
	}
	if *salida != "" {
		b, _ := json.MarshalIndent(map[string]any{
			"n": *n, "tipo": *tipo, "completados": ok, "fallidos": fallidos, "reintentos": reintentos,
			"envio_ms": envio.Milliseconds(), "total_ms": total.Milliseconds(), "throughput": float64(ok) / total.Seconds(),
			"p50_ms": pct(lat, .5).Milliseconds(), "p95_ms": pct(lat, .95).Milliseconds(), "p99_ms": pct(lat, .99).Milliseconds(),
		}, "", "  ")
		os.WriteFile(*salida, b, 0o644)
	}
	if fallidos > 0 {
		os.Exit(2)
	}
}
