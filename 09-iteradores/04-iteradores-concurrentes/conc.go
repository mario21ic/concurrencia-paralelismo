// Package main: iteradores que usan goroutines por dentro, sin romper el contrato.
//
// Reglas de oro para un iterador concurrente:
//  1. yield se llama SIEMPRE desde la goroutine que recibió la llamada al iterador
//     (no desde goroutines internas), y nunca de forma concurrente.
//  2. El iterador no retorna hasta que todas sus goroutines internas terminaron.
//  3. Si yield devuelve false, hay que cancelar y esperar a esas goroutines.
//  4. Un panic en una goroutine interna se propaga a quien itera.
package main

import (
	"context"
	"iter"
	"sync"
)

// Prefetch hace que el productor corra por delante del consumidor (hasta `buf`
// elementos) para solapar el trabajo de ambos.
func Prefetch[T any](seq iter.Seq[T], buf int) iter.Seq[T] {
	return func(yield func(T) bool) {
		ctx, cancel := context.WithCancel(context.Background())
		ch := make(chan T, buf)
		hecho := make(chan struct{})
		var pan any

		go func() {
			defer close(hecho)
			defer close(ch)
			defer func() { pan = recover() }() // se lee solo después de <-hecho
			for v := range seq {
				select {
				case ch <- v:
				case <-ctx.Done():
					return
				}
			}
		}()

		defer func() { // se ejecuta con break, fin normal o panic del consumidor
			cancel()
			<-hecho // esperar: el iterador no retorna con goroutines vivas
		}()

		for v := range ch {
			if !yield(v) {
				return
			}
		}
		<-hecho
		if pan != nil {
			panic(pan)
		}
	}
}

// ParMap aplica f a cada elemento con como máximo `workers` cálculos
// simultáneos y entrega los resultados EN EL ORDEN ORIGINAL.
//
// Técnica: por cada elemento se crea un "futuro" (channel de 1) que se encola
// en orden; el consumidor los lee por orden. Dos límites independientes:
//   - un semáforo de `workers` plazas acota los CÁLCULOS simultáneos;
//   - la cola de futuros (capacidad `workers`) acota los resultados pendientes de
//     consumir y da contrapresión al origen si el consumidor es lento.
func ParMap[A, B any](seq iter.Seq[A], workers int, f func(A) B) iter.Seq[B] {
	return func(yield func(B) bool) {
		ctx, cancel := context.WithCancel(context.Background())
		futuros := make(chan chan B, workers)
		sem := make(chan struct{}, workers)
		var wg sync.WaitGroup // productor y todos los cálculos
		var pan any
		var panOnce sync.Once

		capturar := func() {
			if r := recover(); r != nil {
				panOnce.Do(func() { pan = r })
				cancel()
			}
		}

		wg.Go(func() {
			defer close(futuros)
			defer capturar()
			for a := range seq {
				select {
				case sem <- struct{}{}: // adquirir plaza de cálculo
				case <-ctx.Done():
					return
				}
				fut := make(chan B, 1)
				select {
				case futuros <- fut: // bloquea si ya hay `workers` resultados pendientes
				case <-ctx.Done():
					return
				}
				wg.Go(func() {
					defer func() { <-sem }() // liberar plaza
					defer capturar()
					fut <- f(a)
				})
			}
		})

		defer func() {
			cancel()
			wg.Wait()
			// los panics del origen o de f se relanzan en el goroutine que itera
		}()

		for fut := range futuros {
			select {
			case r := <-fut:
				if !yield(r) {
					return
				}
			case <-ctx.Done():
				cancel()
				wg.Wait()
				if pan != nil {
					panic(pan)
				}
				return
			}
		}
		wg.Wait()
		if pan != nil {
			panic(pan)
		}
	}
}
