package main

import (
	"hash/maphash"
	"sync"

	"golang.org/x/sys/cpu"
)

// Mapa es la interfaz común de las implementaciones comparadas.
type Mapa interface {
	Load(k int) (int, bool)
	Store(k, v int)
	Delete(k int)
}

// ---- Un único RWMutex ----

type MapaGlobal struct {
	mu sync.RWMutex
	m  map[int]int
}

func NuevoGlobal() *MapaGlobal { return &MapaGlobal{m: map[int]int{}} }
func (g *MapaGlobal) Load(k int) (int, bool) {
	g.mu.RLock()
	v, ok := g.m[k]
	g.mu.RUnlock()
	return v, ok
}
func (g *MapaGlobal) Store(k, v int) { g.mu.Lock(); g.m[k] = v; g.mu.Unlock() }
func (g *MapaGlobal) Delete(k int)   { g.mu.Lock(); delete(g.m, k); g.mu.Unlock() }

// ---- sync.Map ----

type MapaSync struct{ m sync.Map }

func (s *MapaSync) Load(k int) (int, bool) {
	v, ok := s.m.Load(k)
	if !ok {
		return 0, false
	}
	return v.(int), true
}
func (s *MapaSync) Store(k, v int) { s.m.Store(k, v) }
func (s *MapaSync) Delete(k int)   { s.m.Delete(k) }

// ---- Mapa fragmentado (sharded) genérico ----

// fragmento lleva relleno para que dos cerrojos vecinos no compartan línea de caché (false sharing).
type fragmento[K comparable, V any] struct {
	mu sync.RWMutex
	m  map[K]V
	_  cpu.CacheLinePad
}

// Fragmentado reparte las claves entre N mapas independientes, cada uno con su cerrojo: dos
// operaciones sobre claves de fragmentos distintos nunca se bloquean entre sí.
type Fragmentado[K comparable, V any] struct {
	semilla maphash.Seed
	frag    []fragmento[K, V]
	mask    uint64
}

// NuevoFragmentado crea un mapa con `n` fragmentos (se redondea a potencia de 2).
func NuevoFragmentado[K comparable, V any](n int) *Fragmentado[K, V] {
	p := 1
	for p < n {
		p <<= 1
	}
	f := &Fragmentado[K, V]{semilla: maphash.MakeSeed(), frag: make([]fragmento[K, V], p), mask: uint64(p - 1)}
	for i := range f.frag {
		f.frag[i].m = map[K]V{}
	}
	return f
}

func (f *Fragmentado[K, V]) de(k K) *fragmento[K, V] {
	return &f.frag[maphash.Comparable(f.semilla, k)&f.mask]
}

func (f *Fragmentado[K, V]) Load(k K) (v V, ok bool) {
	s := f.de(k)
	s.mu.RLock()
	v, ok = s.m[k]
	s.mu.RUnlock()
	return
}

func (f *Fragmentado[K, V]) Store(k K, v V) {
	s := f.de(k)
	s.mu.Lock()
	s.m[k] = v
	s.mu.Unlock()
}

func (f *Fragmentado[K, V]) Delete(k K) {
	s := f.de(k)
	s.mu.Lock()
	delete(s.m, k)
	s.mu.Unlock()
}

// Update hace una lectura-modificación-escritura ATÓMICA sobre la clave (lo que Load+Store no puede):
// fn recibe el valor actual (y si existía) y devuelve el nuevo; si borrar es true se elimina la clave.
func (f *Fragmentado[K, V]) Update(k K, fn func(actual V, existe bool) (nuevo V, borrar bool)) {
	s := f.de(k)
	s.mu.Lock()
	defer s.mu.Unlock()
	actual, existe := s.m[k]
	nuevo, borrar := fn(actual, existe)
	if borrar {
		delete(s.m, k)
	} else {
		s.m[k] = nuevo
	}
}

// LoadOrStore devuelve el valor existente o guarda y devuelve v.
func (f *Fragmentado[K, V]) LoadOrStore(k K, v V) (actual V, cargado bool) {
	s := f.de(k)
	s.mu.Lock()
	defer s.mu.Unlock()
	if a, ok := s.m[k]; ok {
		return a, true
	}
	s.m[k] = v
	return v, false
}

// Len suma los fragmentos uno a uno: NO es una instantánea global (puede haber cambios entre medias).
func (f *Fragmentado[K, V]) Len() (n int) {
	for i := range f.frag {
		f.frag[i].mu.RLock()
		n += len(f.frag[i].m)
		f.frag[i].mu.RUnlock()
	}
	return
}

// Range recorre una copia de cada fragmento por separado; así fn puede llamar al mapa sin
// interbloquearse. Es consistente por fragmento, no globalmente.
func (f *Fragmentado[K, V]) Range(fn func(K, V) bool) {
	for i := range f.frag {
		s := &f.frag[i]
		s.mu.RLock()
		copia := make(map[K]V, len(s.m))
		for k, v := range s.m {
			copia[k] = v
		}
		s.mu.RUnlock()
		for k, v := range copia {
			if !fn(k, v) {
				return
			}
		}
	}
}

// Adaptador al tipo Mapa (claves int) para las pruebas comparativas.
type FragInt struct{ *Fragmentado[int, int] }

func NuevoFragInt(n int) FragInt { return FragInt{NuevoFragmentado[int, int](n)} }
