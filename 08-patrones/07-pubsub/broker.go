// Package main: broker publicador/suscriptor en memoria.
package main

import (
	"context"
	"sync"
	"sync/atomic"
)

// Suscripcion es el lado receptor. C se cierra al cancelar o al cerrar el broker.
type Suscripcion[T any] struct {
	C           <-chan T
	ch          chan T
	broker      *Broker[T]
	descartados atomic.Int64
	detener     func() bool // desregistra el AfterFunc de contexto
}

// Descartados es el número de mensajes que no cupieron en su buffer.
func (s *Suscripcion[T]) Descartados() int64 { return s.descartados.Load() }

// Cancelar da de baja la suscripción (idempotente).
func (s *Suscripcion[T]) Cancelar() {
	if s.detener != nil {
		s.detener()
	}
	s.broker.baja(s)
}

// Broker difunde cada mensaje a TODOS los suscriptores actuales.
//
// Política ante un suscriptor lento: se DESCARTA el mensaje para él (y se
// cuenta). Un suscriptor lento nunca frena a los demás ni al publicador.
type Broker[T any] struct {
	mu      sync.RWMutex
	subs    map[*Suscripcion[T]]struct{}
	cerrado bool
}

func NuevoBroker[T any]() *Broker[T] {
	return &Broker[T]{subs: make(map[*Suscripcion[T]]struct{})}
}

// Suscribir registra un suscriptor con un buffer de `buf` mensajes.
// Si ctx termina, se da de baja automáticamente.
func (b *Broker[T]) Suscribir(ctx context.Context, buf int) *Suscripcion[T] {
	ch := make(chan T, buf)
	s := &Suscripcion[T]{C: ch, ch: ch, broker: b}

	b.mu.Lock()
	if b.cerrado {
		close(ch)
		b.mu.Unlock()
		return s
	}
	b.subs[s] = struct{}{}
	b.mu.Unlock()

	s.detener = context.AfterFunc(ctx, func() { b.baja(s) })
	return s
}

func (b *Broker[T]) baja(s *Suscripcion[T]) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.subs[s]; ok {
		delete(b.subs, s)
		close(s.ch)
	}
}

// Publicar envía v a todos y devuelve a cuántos se entregó.
// Toma RLock mientras envía: como los envíos NO bloquean (select+default) la
// sección es corta, y baja()/Cerrar() (que toman Lock) no pueden cerrar un
// channel mientras alguien le está enviando.
func (b *Broker[T]) Publicar(v T) (entregados int) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.cerrado {
		return 0
	}
	for s := range b.subs {
		select {
		case s.ch <- v:
			entregados++
		default:
			s.descartados.Add(1)
		}
	}
	return entregados
}

// Cerrar da de baja a todos y rechaza nuevas suscripciones y publicaciones.
func (b *Broker[T]) Cerrar() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cerrado {
		return
	}
	b.cerrado = true
	for s := range b.subs {
		close(s.ch)
	}
	clear(b.subs)
}

// Suscriptores devuelve cuántas suscripciones hay activas.
func (b *Broker[T]) Suscriptores() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subs)
}
