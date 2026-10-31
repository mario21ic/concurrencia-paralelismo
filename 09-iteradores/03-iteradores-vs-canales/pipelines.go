package pipelines

import (
	"iter"
	"slices"
)

const N = 100_000

func Datos() []int {
	d := make([]int, N)
	for i := range d {
		d[i] = i
	}
	return d
}

// Bucle clásico: la línea base.
func SumaBucle(d []int) (s int) {
	for _, v := range d {
		if v%2 == 0 {
			s += v * 2
		}
	}
	return
}

// Iteradores componibles (síncronos, en la misma goroutine).
func mapear[A, B any](s iter.Seq[A], f func(A) B) iter.Seq[B] {
	return func(yield func(B) bool) {
		for v := range s {
			if !yield(f(v)) {
				return
			}
		}
	}
}

func filtrar[T any](s iter.Seq[T], p func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range s {
			if p(v) && !yield(v) {
				return
			}
		}
	}
}

func SumaIter(d []int) (s int) {
	pipeline := mapear(filtrar(slices.Values(d), func(n int) bool { return n%2 == 0 }), func(n int) int { return n * 2 })
	for v := range pipeline {
		s += v
	}
	return
}

// Iteradores consumidos con Pull (cada next() cambia de corrutina).
func SumaPull(d []int) (s int) {
	pipeline := mapear(filtrar(slices.Values(d), func(n int) bool { return n%2 == 0 }), func(n int) int { return n * 2 })
	next, stop := iter.Pull(pipeline)
	defer stop()
	for {
		v, ok := next()
		if !ok {
			return
		}
		s += v
	}
}

// Channels: una goroutine por etapa.
func SumaCanales(d []int, buf int) (s int) {
	gen := make(chan int, buf)
	pares := make(chan int, buf)
	dobles := make(chan int, buf)

	go func() {
		defer close(gen)
		for _, v := range d {
			gen <- v
		}
	}()
	go func() {
		defer close(pares)
		for v := range gen {
			if v%2 == 0 {
				pares <- v
			}
		}
	}()
	go func() {
		defer close(dobles)
		for v := range pares {
			dobles <- v * 2
		}
	}()
	for v := range dobles {
		s += v
	}
	return
}
