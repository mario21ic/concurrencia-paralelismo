package main

import (
	"slices"
	"sync"
)

// Conjunto es lo que comparten las dos implementaciones del benchmark.
type Conjunto interface {
	Add(int) bool
	Remove(int) bool
	Contains(int) bool
}

// SliceMutex: slice ordenado + RWMutex. Búsqueda O(log n), inserción/borrado O(n) (memmove) y
// todo serializado por un único cerrojo: la alternativa "simple" con la que comparar.
type SliceMutex struct {
	mu sync.RWMutex
	s  []int
}

func (c *SliceMutex) Contains(k int) bool {
	c.mu.RLock()
	_, ok := slices.BinarySearch(c.s, k)
	c.mu.RUnlock()
	return ok
}

func (c *SliceMutex) Add(k int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	i, ok := slices.BinarySearch(c.s, k)
	if ok {
		return false
	}
	c.s = slices.Insert(c.s, i, k)
	return true
}

func (c *SliceMutex) Remove(k int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	i, ok := slices.BinarySearch(c.s, k)
	if !ok {
		return false
	}
	c.s = slices.Delete(c.s, i, i+1)
	return true
}
