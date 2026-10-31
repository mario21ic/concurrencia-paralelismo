// Package main: tres utilidades clásicas para componer channels.
package main

import "context"

// OrDone envuelve un channel para poder leerlo con un simple `range` sin
// quedarse bloqueado cuando ctx se cancela.
//
//	for v := range OrDone(ctx, in) { ... }   // en vez de un select en cada lectura
func OrDone[T any](ctx context.Context, in <-chan T) <-chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case v, ok := <-in:
				if !ok {
					return
				}
				select {
				case out <- v:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out
}

// Tee duplica un channel en dos: cada valor llega a AMBAS salidas.
// Avanza al ritmo de la salida más lenta (no hay buffer).
func Tee[T any](ctx context.Context, in <-chan T) (<-chan T, <-chan T) {
	o1, o2 := make(chan T), make(chan T)
	go func() {
		defer close(o1)
		defer close(o2)
		for v := range OrDone(ctx, in) {
			a, b := o1, o2 // copias locales: se ponen a nil al entregar
			for range 2 {
				select {
				case a <- v:
					a = nil // ya entregado a o1: apagar ese caso
				case b <- v:
					b = nil
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return o1, o2
}

// Bridge aplana un channel de channels en un único channel, leyendo cada
// sub-channel en orden de llegada.
func Bridge[T any](ctx context.Context, flujo <-chan <-chan T) <-chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		for {
			var sub <-chan T
			select {
			case s, ok := <-flujo:
				if !ok {
					return
				}
				sub = s
			case <-ctx.Done():
				return
			}
			for v := range OrDone(ctx, sub) {
				select {
				case out <- v:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return out
}
