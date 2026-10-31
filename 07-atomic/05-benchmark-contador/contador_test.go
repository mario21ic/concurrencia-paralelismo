package contador

import (
	"sync"
	"sync/atomic"
	"testing"
)

// Mutex: la sección crítica protege un int.
func BenchmarkMutex(b *testing.B) {
	var mu sync.Mutex
	n := 0
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mu.Lock()
			n++
			mu.Unlock()
		}
	})
}

// Atomic: una instrucción atómica por incremento.
func BenchmarkAtomic(b *testing.B) {
	var n atomic.Int64
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			n.Add(1)
		}
	})
}

// Channel (CSP): una goroutine dueña del contador recibe cada incremento.
func BenchmarkChannel(b *testing.B) {
	inc := make(chan struct{}, 128) // con buffer para no medir solo el rendezvous
	fin := make(chan int)
	go func() {
		n := 0
		for range inc {
			n++
		}
		fin <- n
	}()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			inc <- struct{}{}
		}
	})
	close(inc)
	<-fin
}

// Agregación local: cada goroutine cuenta en una variable propia y suma UNA vez
// al final. Elimina la contención en lugar de abaratarla.
func BenchmarkAgregacionLocal(b *testing.B) {
	var total atomic.Int64
	b.RunParallel(func(pb *testing.PB) {
		local := 0
		for pb.Next() {
			local++
		}
		total.Add(int64(local))
	})
}

// Línea base sin contención: un solo hilo, sin sincronización.
func BenchmarkSecuencialSinSync(b *testing.B) {
	n := 0
	for b.Loop() {
		n++
	}
	_ = n
}

// Un solo hilo con sincronización: coste "puro" sin contención.
func BenchmarkMutexSinContencion(b *testing.B) {
	var mu sync.Mutex
	n := 0
	for b.Loop() {
		mu.Lock()
		n++
		mu.Unlock()
	}
}

func BenchmarkAtomicSinContencion(b *testing.B) {
	var n atomic.Int64
	for b.Loop() {
		n.Add(1)
	}
}
