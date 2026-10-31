// Package main: worker pool genérico de tamaño fijo.
package main

import (
	"context"
	"sync"
)

// Resultado conserva el orden de las entradas: Resultado[i] corresponde a entradas[i].
type Resultado[O any] struct {
	Valor O
	Err   error
}

// Procesar aplica f a cada entrada con como máximo `workers` goroutines.
//
//   - Número de goroutines acotado, sin importar len(entradas).
//   - Si ctx se cancela, las entradas no procesadas quedan con Err = ctx.Err().
//   - Los resultados salen en el orden de entrada aunque se terminen desordenados.
//   - Cada worker escribe solo en su posición res[i]: no hace falta mutex.
func Procesar[I, O any](ctx context.Context, workers int, entradas []I, f func(context.Context, I) (O, error)) []Resultado[O] {
	workers = max(1, min(workers, len(entradas)))
	res := make([]Resultado[O], len(entradas))
	trabajos := make(chan int)

	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for i := range trabajos {
				if err := ctx.Err(); err != nil { // cancelado: no empezar más trabajo
					res[i].Err = err
					continue
				}
				v, err := f(ctx, entradas[i])
				res[i] = Resultado[O]{v, err}
			}
		})
	}

alimentar:
	for i := range entradas {
		select {
		case trabajos <- i:
		case <-ctx.Done():
			for j := i; j < len(entradas); j++ { // lo no enviado queda cancelado
				res[j].Err = ctx.Err()
			}
			break alimentar
		}
	}
	close(trabajos)
	wg.Wait()
	return res
}
