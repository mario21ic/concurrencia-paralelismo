// Package main: pipeline con errgroup. El primer error cancela TODAS las etapas.
package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
)

// SumarLineas lee líneas, las convierte a enteros con `workers` goroutines y
// suma el resultado. Si cualquier línea es inválida, todo el pipeline se detiene
// y se devuelve ese error.
//
//	generador ──► [lineas] ──► N parseadores ──► [nums] ──► sumador
func SumarLineas(ctx context.Context, lineas []string, workers int, procesadas *atomic.Int64) (int, error) {
	g, ctx := errgroup.WithContext(ctx)
	chLineas := make(chan string)
	chNums := make(chan int)

	// Etapa 1: generador (dueño del cierre de chLineas).
	g.Go(func() error {
		defer close(chLineas)
		for _, l := range lineas {
			select {
			case chLineas <- l:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	})

	// Etapa 2: N parseadores. Un sub-grupo permite saber cuándo TODOS terminaron
	// para cerrar chNums (varios emisores => un coordinador cierra).
	g.Go(func() error {
		defer close(chNums)
		gp, ctxp := errgroup.WithContext(ctx)
		for range workers {
			gp.Go(func() error {
				for l := range chLineas {
					n, err := strconv.Atoi(strings.TrimSpace(l))
					if err != nil {
						return fmt.Errorf("línea %q: %w", l, err)
					}
					if procesadas != nil {
						procesadas.Add(1)
					}
					select {
					case chNums <- n:
					case <-ctxp.Done():
						return ctxp.Err()
					}
				}
				return nil
			})
		}
		return gp.Wait()
	})

	// Etapa 3: sumador.
	total := 0
	g.Go(func() error {
		for {
			select {
			case n, ok := <-chNums:
				if !ok {
					return nil
				}
				total += n
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	})

	if err := g.Wait(); err != nil {
		return 0, err
	}
	return total, nil
}
