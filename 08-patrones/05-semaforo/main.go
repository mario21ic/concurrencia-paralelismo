// Ejercicio 5: semáforos para limitar concurrencia.
package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/semaphore"
)

// observador mide cuántas tareas corren simultáneamente.
type observador struct{ actuales, maximo atomic.Int32 }

func (o *observador) entrar() {
	a := o.actuales.Add(1)
	for {
		m := o.maximo.Load()
		if a <= m || o.maximo.CompareAndSwap(m, a) {
			return
		}
	}
}
func (o *observador) salir() { o.actuales.Add(-1) }

func main() {
	// 1) Semáforo con un channel con buffer: la capacidad es el límite.
	const limite, tareas = 4, 20
	sem := make(chan struct{}, limite)
	var obs observador
	var wg sync.WaitGroup
	ini := time.Now()
	for range tareas {
		sem <- struct{}{} // adquirir: bloquea si ya hay `limite` en curso
		wg.Go(func() {
			defer func() { <-sem }() // liberar
			obs.entrar()
			defer obs.salir()
			time.Sleep(30 * time.Millisecond)
		})
	}
	wg.Wait()
	fmt.Printf("channel: %d tareas de 30ms, límite %d -> máx. simultáneas=%d, tiempo=%v (ideal 150ms)\n",
		tareas, limite, obs.maximo.Load(), time.Since(ini).Round(10*time.Millisecond))

	// 2) semaphore.Weighted: cada tarea pesa distinto (p. ej. memoria que necesita).
	ctx := context.Background()
	pesos := semaphore.NewWeighted(4)
	var enCurso atomic.Int32
	var maxPeso atomic.Int32
	trabajar := func(nombre string, peso int64, d time.Duration) {
		if err := pesos.Acquire(ctx, peso); err != nil {
			return
		}
		defer pesos.Release(peso)
		cur := enCurso.Add(int32(peso))
		for {
			m := maxPeso.Load()
			if cur <= m || maxPeso.CompareAndSwap(m, cur) {
				break
			}
		}
		time.Sleep(d)
		enCurso.Add(-int32(peso))
	}
	ini = time.Now()
	for i := range 8 {
		peso := int64(1)
		if i%4 == 0 {
			peso = 3 // tareas pesadas
		}
		wg.Go(func() { trabajar(fmt.Sprint("t", i), peso, 40*time.Millisecond) })
	}
	wg.Wait()
	fmt.Printf("weighted: capacidad 4, peso máximo en uso=%d (nunca >4), tiempo=%v\n", maxPeso.Load(), time.Since(ini).Round(10*time.Millisecond))

	// 3) TryAcquire (no bloqueante) y Acquire con plazo.
	s := semaphore.NewWeighted(2)
	fmt.Println("\nTryAcquire(2):", s.TryAcquire(2))
	fmt.Println("TryAcquire(1) con el semáforo lleno:", s.TryAcquire(1))

	ctxPlazo, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	err := s.Acquire(ctxPlazo, 1)
	fmt.Println("Acquire con plazo de 50ms:", err, "| es DeadlineExceeded:", errors.Is(err, context.DeadlineExceeded))

	// Pedir más que la capacidad total nunca podrá satisfacerse.
	ctxPlazo2, cancel2 := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel2()
	fmt.Println("Acquire(3) con capacidad 2:", s.Acquire(ctxPlazo2, 3), "(imposible: solo sale por el plazo)")

	s.Release(2)
	fmt.Println("tras Release(2), TryAcquire(1):", s.TryAcquire(1))
}
