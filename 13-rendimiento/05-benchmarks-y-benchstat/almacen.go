// Package almacen: dos implementaciones de un almacén clave-valor concurrente para
// practicar la comparación rigurosa de benchmarks.
package almacen

import (
	"hash/maphash"
	"sync"
)

// Almacen es lo que ambas implementaciones ofrecen.
type Almacen interface {
	Leer(k uint64) (uint64, bool)
	Escribir(k, v uint64)
}

// ---- "Antes": un mapa con un RWMutex ----

type ConRWMutex struct {
	mu sync.RWMutex
	m  map[uint64]uint64
}

func NuevoRW() *ConRWMutex { return &ConRWMutex{m: make(map[uint64]uint64)} }

func (a *ConRWMutex) Leer(k uint64) (uint64, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	v, ok := a.m[k]
	return v, ok
}

func (a *ConRWMutex) Escribir(k, v uint64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.m[k] = v
}

// ---- "Después": N fragmentos, cada uno con su RWMutex ----

const fragmentos = 64

type fragmento struct {
	mu sync.RWMutex
	m  map[uint64]uint64
	_  [64]byte // relleno (ver ejercicio 2)
}

type Fragmentado struct {
	semilla maphash.Seed
	f       [fragmentos]fragmento
}

func NuevoFragmentado() *Fragmentado {
	a := &Fragmentado{semilla: maphash.MakeSeed()}
	for i := range a.f {
		a.f[i].m = make(map[uint64]uint64)
	}
	return a
}

func (a *Fragmentado) indice(k uint64) int {
	return int(maphash.Comparable(a.semilla, k) & (fragmentos - 1))
}

func (a *Fragmentado) Leer(k uint64) (uint64, bool) {
	f := &a.f[a.indice(k)]
	f.mu.RLock()
	defer f.mu.RUnlock()
	v, ok := f.m[k]
	return v, ok
}

func (a *Fragmentado) Escribir(k, v uint64) {
	f := &a.f[a.indice(k)]
	f.mu.Lock()
	defer f.mu.Unlock()
	f.m[k] = v
}
