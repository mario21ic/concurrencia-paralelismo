package main

import "iter"

// Contar emite desde, desde+1, ... (secuencia infinita y perezosa).
func Contar(desde int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := desde; ; i++ {
			if !yield(i) { // false => el consumidor hizo break: DEBEMOS parar
				return
			}
		}
	}
}

// Fib emite la sucesión de Fibonacci.
func Fib() iter.Seq[int] {
	return func(yield func(int) bool) {
		a, b := 0, 1
		for yield(a) {
			a, b = b, a+b
		}
	}
}

func Map[A, B any](s iter.Seq[A], f func(A) B) iter.Seq[B] {
	return func(yield func(B) bool) {
		for v := range s {
			if !yield(f(v)) {
				return
			}
		}
	}
}

func Filter[T any](s iter.Seq[T], pred func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range s {
			if pred(v) && !yield(v) {
				return
			}
		}
	}
}

func Take[T any](s iter.Seq[T], n int) iter.Seq[T] {
	return func(yield func(T) bool) {
		if n <= 0 {
			return
		}
		i := 0
		for v := range s {
			if !yield(v) {
				return
			}
			if i++; i >= n {
				return
			}
		}
	}
}

// Enumerar convierte una Seq en Seq2 con el índice.
func Enumerar[T any](s iter.Seq[T]) iter.Seq2[int, T] {
	return func(yield func(int, T) bool) {
		i := 0
		for v := range s {
			if !yield(i, v) {
				return
			}
			i++
		}
	}
}
