package main

import (
	"context"
	"iter"
)

// DeCanal adapta un channel a iterador. Al hacer break simplemente deja de leer:
// NO cierra ni vacía el channel. Quien lo produce sigue siendo responsable de
// terminar (cancelación), o se quedará bloqueado enviando.
func DeCanal[T any](ch <-chan T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range ch {
			if !yield(v) {
				return
			}
		}
	}
}

// ACanal vuelca un iterador en un channel desde una goroutine propia.
// Se detiene si ctx se cancela. Devuelve un channel que se cierra al terminar.
//
// Ojo: aquí el iterador se ejecuta en OTRA goroutine, lo cual es correcto
// porque el iterador es quien llama a yield, y lo hace desde esa goroutine.
func ACanal[T any](ctx context.Context, seq iter.Seq[T]) <-chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		for v := range seq {
			select {
			case out <- v:
			case <-ctx.Done():
				return // el break implícito detiene a seq
			}
		}
	}()
	return out
}

// Pagina es una respuesta de una API paginada.
type Pagina[T any] struct {
	Items     []T
	Siguiente bool
}

// Elementos recorre una API paginada de forma PEREZOSA: pide la página
// siguiente solo cuando el consumidor ya agotó la anterior. Los errores
// (incluida la cancelación del contexto) se entregan como segundo valor.
func Elementos[T any](ctx context.Context, pedir func(ctx context.Context, n int) (Pagina[T], error)) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		var cero T
		for n := 0; ; n++ {
			if err := ctx.Err(); err != nil {
				yield(cero, err)
				return
			}
			p, err := pedir(ctx, n)
			if err != nil {
				yield(cero, err)
				return
			}
			for _, it := range p.Items {
				if !yield(it, nil) {
					return // el consumidor no quiere más: no pedir más páginas
				}
			}
			if !p.Siguiente {
				return
			}
		}
	}
}
