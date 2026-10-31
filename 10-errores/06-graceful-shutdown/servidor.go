// Package main: servidor HTTP con apagado ordenado coordinado por errgroup.
package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

// Ejecutar arranca el servidor y un worker en segundo plano y los detiene
// juntos cuando ctx termina o cuando cualquiera de ellos falla.
//
//   - Al cancelarse ctx: se deja de aceptar conexiones y se espera a las
//     peticiones en curso hasta `gracia`.
//   - Si pasa `gracia`, se cierran las conexiones a la fuerza.
//   - Devuelve nil en un apagado limpio.
func Ejecutar(ctx context.Context, ln net.Listener, h http.Handler, gracia time.Duration, latidos *atomic.Int64) error {
	srv := &http.Server{Handler: h}
	g, gctx := errgroup.WithContext(ctx)

	// Componente 1: el servidor.
	g.Go(func() error {
		err := srv.Serve(ln)
		if errors.Is(err, http.ErrServerClosed) {
			return nil // cierre esperado
		}
		return err // fallo real (p. ej. el listener murió) => cancela a los demás
	})

	// Componente 2: apagado coordinado. Se activa al terminar gctx, sea por la
	// señal (ctx) o porque otro componente falló.
	g.Go(func() error {
		<-gctx.Done()
		// El plazo de gracia NO puede heredar la cancelación (ya está cancelado).
		sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), gracia)
		defer cancel()
		if err := srv.Shutdown(sctx); err != nil {
			srv.Close() // se acabó la gracia: cortar las conexiones que quedan
			return err
		}
		return nil
	})

	// Componente 3: un worker en segundo plano.
	g.Go(func() error {
		t := time.NewTicker(10 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				if latidos != nil {
					latidos.Add(1)
				}
			case <-gctx.Done():
				log.Println("worker de fondo detenido")
				return nil
			}
		}
	})

	return g.Wait()
}
