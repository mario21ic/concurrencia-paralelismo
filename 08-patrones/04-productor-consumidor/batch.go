package main

import (
	"context"
	"time"
)

// Batch agrupa elementos en lotes de hasta `tam`. Emite el lote cuando se llena
// o cuando pasan `espera` desde el primer elemento del lote, lo que ocurra antes.
func Batch[T any](ctx context.Context, in <-chan T, tam int, espera time.Duration) <-chan []T {
	out := make(chan []T)
	go func() {
		defer close(out)
		var lote []T
		var temporizador *time.Timer
		var vence <-chan time.Time // nil mientras no hay lote en curso => caso apagado

		vaciar := func() bool {
			if len(lote) == 0 {
				return true
			}
			if temporizador != nil {
				temporizador.Stop()
			}
			vence = nil
			l := lote
			lote = nil
			select {
			case out <- l:
				return true
			case <-ctx.Done():
				return false
			}
		}

		for {
			select {
			case v, ok := <-in:
				if !ok {
					vaciar()
					return
				}
				lote = append(lote, v)
				if len(lote) == 1 {
					temporizador = time.NewTimer(espera)
					vence = temporizador.C
				}
				if len(lote) >= tam && !vaciar() {
					return
				}
			case <-vence:
				if !vaciar() {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}
