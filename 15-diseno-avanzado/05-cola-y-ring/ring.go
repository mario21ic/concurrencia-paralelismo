package main

import (
	"sync/atomic"

	"golang.org/x/sys/cpu"
)

// ---- Ring SPSC: un productor y un consumidor, sin cerrojos (Lamport) ----

// RingSPSC es un buffer circular de capacidad potencia de 2 para EXACTAMENTE un productor y un
// consumidor. Cada extremo escribe solo su contador; el otro lo lee. No usa CAS ni cerrojos.
// Los contadores van en líneas de caché distintas para evitar false sharing (módulo 13).
type RingSPSC[T any] struct {
	buf  []T
	mask uint64
	_    cpu.CacheLinePad
	cola atomic.Uint64 // próxima posición a escribir (solo la modifica el productor)
	vCab uint64        // copia que el productor tiene de `cab` (solo la toca el productor)
	_    cpu.CacheLinePad
	cab  atomic.Uint64 // próxima posición a leer (solo la modifica el consumidor)
	vCol uint64        // copia que el consumidor tiene de `cola` (solo la toca el consumidor)
	_    cpu.CacheLinePad
}

func NuevoRingSPSC[T any](capacidad int) *RingSPSC[T] {
	return &RingSPSC[T]{buf: make([]T, redondear(capacidad)), mask: uint64(redondear(capacidad)) - 1}
}

// TryPush solo relee el contador del consumidor cuando, según su copia, el anillo parece lleno:
// así la línea de caché de `cab` no viaja entre núcleos en cada inserción.
func (r *RingSPSC[T]) TryPush(v T) bool {
	t := r.cola.Load()
	if t-r.vCab == uint64(len(r.buf)) {
		r.vCab = r.cab.Load()
		if t-r.vCab == uint64(len(r.buf)) {
			return false // lleno
		}
	}
	r.buf[t&r.mask] = v
	r.cola.Store(t + 1) // publica: el Store atómico ordena la escritura anterior (happens-before)
	return true
}

func (r *RingSPSC[T]) TryPop() (v T, ok bool) {
	h := r.cab.Load()
	if h == r.vCol {
		r.vCol = r.cola.Load()
		if h == r.vCol {
			return v, false // vacío
		}
	}
	v = r.buf[h&r.mask]
	var cero T
	r.buf[h&r.mask] = cero
	r.cab.Store(h + 1)
	return v, true
}

// ---- Ring MPMC acotado de Vyukov: varios productores y consumidores, con CAS por posición ----

type celda[T any] struct {
	sec atomic.Uint64 // número de secuencia: indica de quién es el turno sobre la celda
	val T
}

// RingMPMC reparte el trabajo con una secuencia por celda: un productor reclama una posición con
// CAS sobre `cola` y solo escribe cuando la celda está libre para su vuelta; un consumidor, igual.
type RingMPMC[T any] struct {
	celdas []celda[T]
	mask   uint64
	_      cpu.CacheLinePad
	cola   atomic.Uint64
	_      cpu.CacheLinePad
	cab    atomic.Uint64
	_      cpu.CacheLinePad
}

func NuevoRingMPMC[T any](capacidad int) *RingMPMC[T] {
	n := redondear(capacidad)
	r := &RingMPMC[T]{celdas: make([]celda[T], n), mask: uint64(n) - 1}
	for i := range r.celdas {
		r.celdas[i].sec.Store(uint64(i))
	}
	return r
}

func (r *RingMPMC[T]) TryPush(v T) bool {
	for {
		pos := r.cola.Load()
		c := &r.celdas[pos&r.mask]
		dif := int64(c.sec.Load()) - int64(pos)
		switch {
		case dif == 0: // la celda está libre para esta vuelta: intento reclamarla
			if r.cola.CompareAndSwap(pos, pos+1) {
				c.val = v
				c.sec.Store(pos + 1) // publica: ya hay dato para el consumidor
				return true
			}
		case dif < 0:
			return false // lleno: la celda aún tiene el dato de la vuelta anterior
		}
		// dif > 0: otro productor se me adelantó; recargo pos y reintento
	}
}

func (r *RingMPMC[T]) TryPop() (v T, ok bool) {
	for {
		pos := r.cab.Load()
		c := &r.celdas[pos&r.mask]
		dif := int64(c.sec.Load()) - int64(pos+1)
		switch {
		case dif == 0:
			if r.cab.CompareAndSwap(pos, pos+1) {
				v = c.val
				var cero T
				c.val = cero
				c.sec.Store(pos + r.mask + 1) // libera la celda para la vuelta siguiente
				return v, true
			}
		case dif < 0:
			return v, false // vacío
		}
	}
}

func redondear(n int) int {
	p := 1
	for p < n {
		p <<= 1
	}
	return p
}
