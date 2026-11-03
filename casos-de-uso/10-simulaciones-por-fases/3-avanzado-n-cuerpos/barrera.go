package main

import "sync"

// Barrera es una barrera CÍCLICA reutilizable: n goroutines llaman a Esperar y ninguna sigue hasta que
// han llegado las n; después la barrera queda lista para la siguiente ronda.
//
// El contador de generación evita el error clásico de una barrera de un solo uso: una goroutine rápida
// que ya dio la vuelta y llama a Esperar de nuevo no debe despertar por error con el Broadcast de la
// ronda anterior, ni colarse en ella.
type Barrera struct {
	mu         sync.Mutex
	cond       *sync.Cond
	n          int
	esperando  int
	generacion int
}

func NuevaBarrera(n int) *Barrera {
	b := &Barrera{n: n}
	b.cond = sync.NewCond(&b.mu)
	return b
}

func (b *Barrera) Esperar() {
	b.mu.Lock()
	defer b.mu.Unlock()
	gen := b.generacion
	b.esperando++
	if b.esperando == b.n { // el último en llegar abre la barrera para todos
		b.esperando = 0
		b.generacion++
		b.cond.Broadcast()
		return
	}
	for gen == b.generacion { // bucle: Wait puede despertar sin que la ronda haya terminado
		b.cond.Wait()
	}
}
