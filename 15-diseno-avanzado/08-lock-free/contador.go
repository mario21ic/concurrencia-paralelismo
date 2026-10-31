package main

import (
	"sync"
	"sync/atomic"
)

// Tres formas de incrementar un contador compartido.

type ContadorMutex struct {
	mu sync.Mutex
	n  int64
}

func (c *ContadorMutex) Inc() { c.mu.Lock(); c.n++; c.mu.Unlock() }

// ContadorCAS: bucle CAS. Lock-free (alguien siempre progresa) pero NO wait-free: una goroutine
// concreta puede reintentar sin límite. Devuelve cuántos intentos fallidos hizo.
type ContadorCAS struct{ n atomic.Int64 }

func (c *ContadorCAS) Inc() (reintentos int) {
	for {
		v := c.n.Load()
		if c.n.CompareAndSwap(v, v+1) {
			return
		}
		reintentos++
	}
}

// ContadorAdd: una instrucción atómica (LOCK XADD en x86; LDADD en ARM con LSE): wait-free en hardware.
type ContadorAdd struct{ n atomic.Int64 }

func (c *ContadorAdd) Inc() { c.n.Add(1) }
