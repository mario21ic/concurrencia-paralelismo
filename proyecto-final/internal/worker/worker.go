// Package worker ejecuta trabajos arrendados al coordinador: un bucle de arriendo (long polling) que
// reparte los trabajos en un número acotado de ranuras, con tiempo límite por trabajo, latidos que
// renuevan el arriendo, recuperación de pánicos y apagado ordenado con periodo de gracia.
package worker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand/v2"
	"runtime"
	"sync"
	"time"

	"github.com/mario21ic/golang-concurrencia-paralelismo/proyecto-final/internal/api"
	"github.com/mario21ic/golang-concurrencia-paralelismo/proyecto-final/internal/modelo"
)

// Handler ejecuta un tipo de trabajo. Debe respetar ctx (se cancela por tiempo límite, por pérdida del
// arriendo o por apagado forzoso). Devolver un error que envuelva ErrPermanente evita los reintentos.
type Handler func(ctx context.Context, payload json.RawMessage) (json.RawMessage, error)

// ErrPermanente marca un fallo que no se arregla reintentando (datos inválidos, tipo desconocido…).
var ErrPermanente = errors.New("error permanente")

var errArriendoPerdido = errors.New("arriendo perdido")

// Config del worker.
type Config struct {
	ID     string
	Slots  int           // trabajos simultáneos; defecto GOMAXPROCS (que en un contenedor respeta el límite de CPU)
	Espera time.Duration // duración de cada sondeo largo; defecto 5 s
	Gracia time.Duration // al apagar, cuánto se espera a los trabajos en curso; defecto 10 s
	Log    *slog.Logger
}

type Worker struct {
	cfg      Config
	cli      *api.Cliente
	handlers map[string]Handler
}

func New(cli *api.Cliente, cfg Config, handlers map[string]Handler) *Worker {
	if cfg.Slots <= 0 {
		cfg.Slots = runtime.GOMAXPROCS(0)
	}
	if cfg.Espera <= 0 {
		cfg.Espera = 5 * time.Second
	}
	if cfg.Gracia <= 0 {
		cfg.Gracia = 10 * time.Second
	}
	if cfg.Log == nil {
		cfg.Log = slog.New(slog.DiscardHandler)
	}
	return &Worker{cfg: cfg, cli: cli, handlers: handlers}
}

// Run arrienda y ejecuta trabajos hasta que ctx se cancela. Entonces deja de arrendar, espera hasta
// `Gracia` a que terminen los trabajos en curso y cancela los que queden (que se devuelven a la cola
// con Nack reintentable). Al volver, no queda ninguna goroutine del worker.
func (w *Worker) Run(ctx context.Context) error {
	// Los trabajos NO cuelgan de ctx: al apagar se les da una gracia antes de cancelarlos.
	trabajoCtx, cancelTrabajos := context.WithCancel(context.WithoutCancel(ctx))
	defer cancelTrabajos()
	libres := make(chan struct{}, w.cfg.Slots) // ranuras ocupadas (semáforo)
	var wg sync.WaitGroup
	fallos := 0

bucle:
	for ctx.Err() == nil {
		select { // espera una ranura libre
		case libres <- struct{}{}:
		case <-ctx.Done():
			break bucle
		}
		n := 1
		for n < w.cfg.Slots { // y coge todas las que haya, para arrendar un lote
			select {
			case libres <- struct{}{}:
				n++
				continue
			default:
			}
			break
		}
		jobs, err := w.cli.Arrendar(ctx, w.cfg.ID, n, w.cfg.Espera)
		for i := len(jobs); i < n; i++ { // devuelve las ranuras que no se usarán
			<-libres
		}
		if err != nil {
			if ctx.Err() != nil {
				break
			}
			fallos++
			espera := min(100*time.Millisecond<<min(fallos, 6), 5*time.Second)
			espera = espera/2 + time.Duration(rand.Int64N(int64(espera/2)+1))
			w.cfg.Log.Warn("no se pudo arrendar", "error", err, "reintento_en", espera.Round(time.Millisecond))
			select {
			case <-time.After(espera):
			case <-ctx.Done():
			}
			continue
		}
		fallos = 0
		for _, j := range jobs {
			wg.Go(func() {
				defer func() { <-libres }()
				w.ejecutar(trabajoCtx, j)
			})
		}
	}

	// Apagado ordenado: gracia para los trabajos en curso y luego cancelación.
	hecho := make(chan struct{})
	go func() { wg.Wait(); close(hecho) }()
	select {
	case <-hecho:
	case <-time.After(w.cfg.Gracia):
		w.cfg.Log.Warn("se agotó la gracia: cancelando los trabajos en curso")
		cancelTrabajos()
		<-hecho
	}
	return nil
}

func (w *Worker) ejecutar(trabajoCtx context.Context, j modelo.Job) {
	log := w.cfg.Log.With("job", j.ID, "tipo", j.Tipo, "intento", j.Intento)
	ctx, cancelCausa := context.WithCancelCause(trabajoCtx)
	defer cancelCausa(nil)
	ctx, cancelPlazo := context.WithTimeout(ctx, time.Duration(j.TimeoutMS)*time.Millisecond)
	defer cancelPlazo()

	// Latidos: mientras el trabajo corre, se renueva el arriendo para que no caduque.
	var latidos sync.WaitGroup
	paraLatidos := make(chan struct{})
	latidos.Go(func() {
		ttl := time.Duration(j.ArriendoMS) * time.Millisecond
		if ttl <= 0 {
			return
		}
		t := time.NewTicker(ttl / 3)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				err := w.cli.Latido(ctx, j.ID, w.cfg.ID)
				if errors.Is(err, modelo.ErrArriendoPerdido) {
					log.Warn("arriendo perdido: se cancela el trabajo")
					cancelCausa(errArriendoPerdido)
					return
				}
			case <-paraLatidos:
				return
			case <-ctx.Done():
				return
			}
		}
	})

	res, err := w.llamar(ctx, j)
	close(paraLatidos)
	latidos.Wait()

	// Informar al coordinador aunque ctx ya esté cancelado (plazo vencido o apagado).
	rep, cancelRep := context.WithTimeout(context.WithoutCancel(trabajoCtx), 5*time.Second)
	defer cancelRep()
	switch {
	case context.Cause(ctx) == errArriendoPerdido:
		// otro worker se hará cargo: no hay nada que informar
	case err == nil:
		if e := w.cli.Ack(rep, j.ID, w.cfg.ID, res); e != nil {
			log.Warn("Ack falló", "error", e)
		}
	default:
		reintentable := !errors.Is(err, ErrPermanente)
		motivo := err.Error()
		if trabajoCtx.Err() != nil {
			motivo = "worker apagándose: " + motivo
		}
		if e := w.cli.Nack(rep, j.ID, w.cfg.ID, motivo, reintentable); e != nil {
			log.Warn("Nack falló", "error", e)
		}
	}
}

// llamar ejecuta el handler convirtiendo los pánicos en errores (un pánico en una goroutine mataría el
// proceso entero, y con él todos los trabajos del worker).
func (w *Worker) llamar(ctx context.Context, j modelo.Job) (res json.RawMessage, err error) {
	h, ok := w.handlers[j.Tipo]
	if !ok {
		return nil, fmt.Errorf("%w: tipo desconocido %q", ErrPermanente, j.Tipo)
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("pánico en el handler: %v", r)
		}
	}()
	res, err = h(ctx, j.Payload)
	if err == nil && ctx.Err() != nil {
		// terminó, pero llegó tarde: el resultado se descarta igual que si hubiera fallado
		return nil, ctx.Err()
	}
	return res, err
}
