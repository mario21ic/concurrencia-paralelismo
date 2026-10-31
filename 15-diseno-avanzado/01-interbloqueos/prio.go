// Package main (ejercicio 1): un "procesador" con prioridades para estudiar inversión de prioridad
// y starvation. Go NO tiene prioridades de goroutine: la inversión aparece en recursos de la
// aplicación (una cola de turnos, un pool, un semáforo) combinados con un lock.
package main

import (
	"sync"
	"sync/atomic"
	"time"
)

// Prio es la prioridad dinámica de una tarea (mayor = más urgente). Es atómica porque otra
// goroutine puede elevarla (herencia de prioridad).
type Prio = atomic.Int32

// PrioSem es un semáforo que concede el recurso al que espera con MAYOR prioridad efectiva.
// Si Envejecimiento > 0, cada Envejecimiento de espera suma 1 punto (evita la inanición).
type PrioSem struct {
	Envejecimiento time.Duration

	mu     sync.Mutex
	libres int
	cola   []*espera
}

type espera struct {
	prio  *Prio
	desde time.Time
	ok    chan struct{}
}

func NuevoPrioSem(n int) *PrioSem { return &PrioSem{libres: n} }

func (s *PrioSem) Acquire(p *Prio) {
	s.mu.Lock()
	if s.libres > 0 {
		s.libres--
		s.mu.Unlock()
		return
	}
	e := &espera{prio: p, desde: time.Now(), ok: make(chan struct{})}
	s.cola = append(s.cola, e)
	s.mu.Unlock()
	<-e.ok
}

func (s *PrioSem) Release() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.cola) == 0 {
		s.libres++
		return
	}
	mejor, ahora := 0, time.Now()
	for i := range s.cola {
		if s.efectiva(s.cola[i], ahora) > s.efectiva(s.cola[mejor], ahora) {
			mejor = i
		}
	}
	e := s.cola[mejor]
	s.cola = append(s.cola[:mejor], s.cola[mejor+1:]...)
	close(e.ok) // se traspasa el recurso: no se incrementa libres
}

func (s *PrioSem) efectiva(e *espera, ahora time.Time) int64 {
	p := int64(e.prio.Load())
	if s.Envejecimiento > 0 {
		p += int64(ahora.Sub(e.desde) / s.Envejecimiento)
	}
	return p
}

// Mutex es un cerrojo con herencia de prioridad opcional: si un waiter más urgente espera,
// el propietario hereda su prioridad hasta que suelte el cerrojo. (Versión didáctica.)
type Mutex struct {
	Herencia bool

	mu    sync.Mutex
	owner atomic.Pointer[Prio]
	base  atomic.Int32
}

func (m *Mutex) Lock(yo *Prio) {
	if m.mu.TryLock() {
		m.base.Store(yo.Load())
		m.owner.Store(yo)
		return
	}
	if m.Herencia {
		// Elevar al propietario (si ya conocido) a la prioridad del que espera.
		for {
			o := m.owner.Load()
			if o == nil {
				break // el propietario aún no se ha registrado: carrera aceptada en la versión didáctica
			}
			cur := o.Load()
			if cur >= yo.Load() || o.CompareAndSwap(cur, yo.Load()) {
				break
			}
		}
	}
	m.mu.Lock()
	m.base.Store(yo.Load())
	m.owner.Store(yo)
}

func (m *Mutex) Unlock() {
	if o := m.owner.Swap(nil); o != nil {
		o.Store(m.base.Load()) // devuelve la prioridad original
	}
	m.mu.Unlock()
}
