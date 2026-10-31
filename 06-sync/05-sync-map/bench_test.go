package main

import (
	"sync"
	"testing"
)

const claves = 1024

type mapaRW struct {
	mu sync.RWMutex
	m  map[int]int
}

func (s *mapaRW) Load(k int) (int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.m[k]
	return v, ok
}
func (s *mapaRW) Store(k, v int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[k] = v
}

// Caso 1: claves estables, casi solo lecturas (el caso ideal de sync.Map).
func BenchmarkLecturaIntensa_SyncMap(b *testing.B) {
	var m sync.Map
	for i := range claves {
		m.Store(i, i)
	}
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			m.Load(i % claves)
			i++
		}
	})
}

func BenchmarkLecturaIntensa_RWMutexMap(b *testing.B) {
	s := &mapaRW{m: map[int]int{}}
	for i := range claves {
		s.Store(i, i)
	}
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			s.Load(i % claves)
			i++
		}
	})
}

// Caso 2: 50 % escrituras sobre pocas claves (mal caso para sync.Map).
func BenchmarkMixto_SyncMap(b *testing.B) {
	var m sync.Map
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				m.Store(i%16, i)
			} else {
				m.Load(i % 16)
			}
			i++
		}
	})
}

func BenchmarkMixto_RWMutexMap(b *testing.B) {
	s := &mapaRW{m: map[int]int{}}
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				s.Store(i%16, i)
			} else {
				s.Load(i % 16)
			}
			i++
		}
	})
}
