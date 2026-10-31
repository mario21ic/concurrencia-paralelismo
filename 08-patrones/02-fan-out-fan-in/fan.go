// Package main: fan-out / fan-in genéricos.
package main

import (
	"context"
	"sync"
)

// Merge (fan-in) combina varios channels en uno. Se cierra cuando todos los
// de entrada se cierran o cuando ctx se cancela. NO preserva el orden.
func Merge[T any](ctx context.Context, ins ...<-chan T) <-chan T {
	out := make(chan T)
	var wg sync.WaitGroup
	for _, in := range ins {
		wg.Go(func() {
			for v := range in {
				select {
				case out <- v:
				case <-ctx.Done():
					return
				}
			}
		})
	}
	go func() { // el coordinador es el único que cierra `out`
		wg.Wait()
		close(out)
	}()
	return out
}

// FanOut lanza n workers que leen del MISMO channel de entrada (cada valor lo
// recibe exactamente un worker) y devuelve sus n salidas.
func FanOut[A, B any](ctx context.Context, in <-chan A, n int, f func(A) B) []<-chan B {
	outs := make([]<-chan B, n)
	for i := range n {
		out := make(chan B)
		outs[i] = out
		go func() {
			defer close(out)
			for v := range in {
				select {
				case out <- f(v):
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	return outs
}

// Generar emite los valores y se detiene si ctx se cancela.
func Generar[T any](ctx context.Context, vals ...T) <-chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		for _, v := range vals {
			select {
			case out <- v:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}
