// Package main: pila lock-free (algoritmo de Treiber) con atomic.Pointer.
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type nodo[T any] struct {
	valor     T
	siguiente *nodo[T]
}

// Pila es una pila LIFO segura para uso concurrente, sin locks.
type Pila[T any] struct {
	cabeza atomic.Pointer[nodo[T]]
}

// Push apila v. Reintenta si otra goroutine cambió la cabeza.
func (p *Pila[T]) Push(v T) {
	n := &nodo[T]{valor: v}
	for {
		viejo := p.cabeza.Load()
		n.siguiente = viejo
		if p.cabeza.CompareAndSwap(viejo, n) {
			return
		}
	}
}

// Pop desapila; ok=false si está vacía.
func (p *Pila[T]) Pop() (v T, ok bool) {
	for {
		viejo := p.cabeza.Load()
		if viejo == nil {
			return v, false
		}
		if p.cabeza.CompareAndSwap(viejo, viejo.siguiente) {
			return viejo.valor, true
		}
	}
}

// pilaMutex: referencia para comparar.
type pilaMutex[T any] struct {
	mu    sync.Mutex
	datos []T
}

func (p *pilaMutex[T]) Push(v T) {
	p.mu.Lock()
	p.datos = append(p.datos, v)
	p.mu.Unlock()
}

func (p *pilaMutex[T]) Pop() (v T, ok bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.datos) == 0 {
		return v, false
	}
	v = p.datos[len(p.datos)-1]
	p.datos = p.datos[:len(p.datos)-1]
	return v, true
}

type pila interface {
	Push(int)
	Pop() (int, bool)
}

// prueba: P productores apilan 1..N cada uno y C consumidores desapilan hasta
// haber sacado todo. Verifica que no se pierde ni duplica ningún elemento.
func prueba(p pila, productores, consumidores, porProductor int) (suma, esperada int64, d time.Duration) {
	total := int64(productores * porProductor)
	for i := 1; i <= porProductor; i++ {
		esperada += int64(i) * int64(productores)
	}

	var sacados, sumaAt atomic.Int64
	var wg sync.WaitGroup
	ini := time.Now()
	for range productores {
		wg.Go(func() {
			for i := 1; i <= porProductor; i++ {
				p.Push(i)
			}
		})
	}
	for range consumidores {
		wg.Go(func() {
			for sacados.Load() < total {
				if v, ok := p.Pop(); ok {
					sacados.Add(1)
					sumaAt.Add(int64(v))
				}
			}
		})
	}
	wg.Wait()
	return sumaAt.Load(), esperada, time.Since(ini)
}

func main() {
	const prod, cons, n = 4, 4, 200_000
	for _, c := range []struct {
		nombre string
		p      pila
	}{
		{"lock-free (CAS)", &Pila[int]{}},
		{"mutex", &pilaMutex[int]{}},
	} {
		s, e, d := prueba(c.p, prod, cons, n)
		fmt.Printf("%-16s suma=%d esperada=%d correcto=%v en %v\n", c.nombre, s, e, s == e, d.Round(time.Millisecond))
	}
}
