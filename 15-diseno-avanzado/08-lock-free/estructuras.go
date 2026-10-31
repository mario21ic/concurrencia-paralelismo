package main

import (
	"sync"
	"sync/atomic"
)

// ---- Pila de Treiber con punteros (segura frente a ABA gracias al GC) ----

type nodoT[T any] struct {
	val  T
	next *nodoT[T]
}

// PilaTreiber: push/pop con un bucle CAS sobre `cabeza`. Es lock-free: si un CAS falla es porque
// OTRA goroutine avanzó. En Go el ABA con punteros no ocurre: mientras una goroutine conserve el
// puntero `viejo`, el GC no puede reutilizar esa memoria para otro nodo.
type PilaTreiber[T any] struct{ cabeza atomic.Pointer[nodoT[T]] }

func (p *PilaTreiber[T]) Push(v T) {
	n := &nodoT[T]{val: v}
	for {
		n.next = p.cabeza.Load()
		if p.cabeza.CompareAndSwap(n.next, n) {
			return
		}
	}
}

func (p *PilaTreiber[T]) Pop() (v T, ok bool) {
	for {
		viejo := p.cabeza.Load()
		if viejo == nil {
			return v, false
		}
		if p.cabeza.CompareAndSwap(viejo, viejo.next) {
			return viejo.val, true
		}
	}
}

// ---- Pila con mutex (referencia) ----

type PilaMutex[T any] struct {
	mu sync.Mutex
	s  []T
}

func (p *PilaMutex[T]) Push(v T) { p.mu.Lock(); p.s = append(p.s, v); p.mu.Unlock() }
func (p *PilaMutex[T]) Pop() (v T, ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.s) == 0 {
		return v, false
	}
	v = p.s[len(p.s)-1]
	p.s = p.s[:len(p.s)-1]
	return v, true
}

// ---- Cola de Michael–Scott (lock-free, MPMC, no acotada) ----

type nodoM[T any] struct {
	val  T
	next atomic.Pointer[nodoM[T]]
}

// ColaMS: lista con un nodo "centinela" inicial. `cabeza` apunta al último nodo ya consumido y
// `cola` al último o penúltimo. Si un productor ve que `cola` va atrasada, la AVANZA él mismo
// (ayuda cooperativa): así ningún hilo lento bloquea a los demás.
type ColaMS[T any] struct {
	cabeza, cola atomic.Pointer[nodoM[T]]
}

func NuevaColaMS[T any]() *ColaMS[T] {
	c := &ColaMS[T]{}
	s := &nodoM[T]{}
	c.cabeza.Store(s)
	c.cola.Store(s)
	return c
}

func (c *ColaMS[T]) Enqueue(v T) {
	n := &nodoM[T]{val: v}
	for {
		cola := c.cola.Load()
		sig := cola.next.Load()
		if cola != c.cola.Load() {
			continue // instantánea incoherente
		}
		if sig != nil {
			c.cola.CompareAndSwap(cola, sig) // ayuda: cola atrasada
			continue
		}
		if cola.next.CompareAndSwap(nil, n) { // enlaza al final (linealización)
			c.cola.CompareAndSwap(cola, n) // intenta avanzar `cola` (si falla, alguien lo hará)
			return
		}
	}
}

func (c *ColaMS[T]) Dequeue() (v T, ok bool) {
	for {
		cab := c.cabeza.Load()
		cola := c.cola.Load()
		sig := cab.next.Load()
		if cab != c.cabeza.Load() {
			continue
		}
		if sig == nil {
			return v, false // vacía
		}
		if cab == cola {
			c.cola.CompareAndSwap(cola, sig) // cola atrasada: ayuda antes de sacar
			continue
		}
		v = sig.val
		if c.cabeza.CompareAndSwap(cab, sig) {
			return v, true
		}
	}
}

// ---- Cola con mutex (referencia) ----

type ColaMutex[T any] struct {
	mu sync.Mutex
	s  []T
}

func (c *ColaMutex[T]) Enqueue(v T) { c.mu.Lock(); c.s = append(c.s, v); c.mu.Unlock() }
func (c *ColaMutex[T]) Dequeue() (v T, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.s) == 0 {
		return v, false
	}
	v = c.s[0]
	c.s = c.s[1:]
	return v, true
}
