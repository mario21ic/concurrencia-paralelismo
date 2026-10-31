package main

import (
	"context"
	"errors"
	"sync"
)

// ErrCerrada: la cola está cerrada (y, en Get, además vacía).
var ErrCerrada = errors.New("cola cerrada")

// Cola es una cola FIFO acotada, segura para muchos productores y consumidores, con
// cancelación por contexto y Close: lo que un channel da, pero sobre un buffer circular propio.
type Cola[T any] struct {
	mu      sync.Mutex
	noVacia *sync.Cond
	noLlena *sync.Cond
	buf     []T
	cabeza  int // índice del próximo elemento a sacar
	n       int
	cerrada bool
}

func NuevaCola[T any](capacidad int) *Cola[T] {
	q := &Cola[T]{buf: make([]T, capacidad)}
	q.noVacia = sync.NewCond(&q.mu)
	q.noLlena = sync.NewCond(&q.mu)
	return q
}

// despertarAlCancelar registra una acción que, si ctx se cancela, despierta a los que esperan
// en c para que reevalúen ctx.Err(). Es el modo idiomático de hacer cancelable un sync.Cond.
func (q *Cola[T]) despertarAlCancelar(ctx context.Context, c *sync.Cond) (detener func() bool) {
	return context.AfterFunc(ctx, func() {
		q.mu.Lock() // sin tomar el cerrojo, el Broadcast podría perderse entre la comprobación y el Wait
		c.Broadcast()
		q.mu.Unlock()
	})
}

// Put bloquea mientras la cola esté llena. Devuelve ctx.Err() si se cancela o ErrCerrada.
func (q *Cola[T]) Put(ctx context.Context, v T) error {
	defer q.despertarAlCancelar(ctx, q.noLlena)()
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.n == len(q.buf) && !q.cerrada && ctx.Err() == nil {
		q.noLlena.Wait()
	}
	switch {
	case q.cerrada:
		return ErrCerrada
	case ctx.Err() != nil:
		return ctx.Err()
	}
	q.buf[(q.cabeza+q.n)%len(q.buf)] = v
	q.n++
	q.noVacia.Signal()
	return nil
}

// TryPut no bloquea: false si está llena o cerrada.
func (q *Cola[T]) TryPut(v T) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.cerrada || q.n == len(q.buf) {
		return false
	}
	q.buf[(q.cabeza+q.n)%len(q.buf)] = v
	q.n++
	q.noVacia.Signal()
	return true
}

// Get bloquea mientras la cola esté vacía. Tras Close entrega lo pendiente y luego ErrCerrada.
func (q *Cola[T]) Get(ctx context.Context) (T, error) {
	var cero T
	defer q.despertarAlCancelar(ctx, q.noVacia)()
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.n == 0 && !q.cerrada && ctx.Err() == nil {
		q.noVacia.Wait()
	}
	if q.n == 0 {
		if q.cerrada {
			return cero, ErrCerrada
		}
		return cero, ctx.Err()
	}
	v := q.buf[q.cabeza]
	q.buf[q.cabeza] = cero // no retener referencias
	q.cabeza = (q.cabeza + 1) % len(q.buf)
	q.n--
	q.noLlena.Signal()
	return v, nil
}

// Close impide nuevos Put y despierta a todos los bloqueados. Es idempotente.
func (q *Cola[T]) Close() {
	q.mu.Lock()
	q.cerrada = true
	q.noVacia.Broadcast()
	q.noLlena.Broadcast()
	q.mu.Unlock()
}

func (q *Cola[T]) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.n
}
