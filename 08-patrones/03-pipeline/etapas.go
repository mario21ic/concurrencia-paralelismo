// Package main: etapas genéricas de un pipeline, todas cancelables.
package main

import (
	"context"
	"strconv"
)

// Contar emite 1, 2, 3, ... hasta que ctx se cancele (fuente infinita).
func Contar(ctx context.Context) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for i := 1; ; i++ {
			select {
			case out <- i:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

// Desde emite los valores dados.
func Desde[T any](ctx context.Context, vals ...T) <-chan T {
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

// Mapear transforma cada elemento.
func Mapear[A, B any](ctx context.Context, in <-chan A, f func(A) B) <-chan B {
	out := make(chan B)
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
	return out
}

// Filtrar deja pasar los elementos que cumplen pred.
func Filtrar[T any](ctx context.Context, in <-chan T, pred func(T) bool) <-chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		for v := range in {
			if !pred(v) {
				continue
			}
			select {
			case out <- v:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

// Tomar deja pasar los n primeros elementos y cierra su salida.
func Tomar[T any](ctx context.Context, in <-chan T, n int) <-chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		for range n {
			select {
			case v, ok := <-in:
				if !ok {
					return
				}
				select {
				case out <- v:
				case <-ctx.Done():
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

// Par transporta un valor O un error por el pipeline: el error viaja como un
// dato más y la etapa final decide qué hacer.
type Par[T any] struct {
	Val T
	Err error
}

// Parsear convierte strings a enteros sin abortar el pipeline ante un error.
func Parsear(ctx context.Context, in <-chan string) <-chan Par[int] {
	out := make(chan Par[int])
	go func() {
		defer close(out)
		for s := range in {
			n, err := strconv.Atoi(s)
			select {
			case out <- Par[int]{n, err}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}
