// Caso 5.1 (básico): no más de N operaciones a la vez (semáforo).
//
// Problema: hay que descargar 20 ficheros, pero el servidor solo tolera 5 conexiones simultáneas (o
// la memoria solo da para 5 a la vez). Lanzar 20 goroutines sin freno lo saturaría.
//
// Tres formas equivalentes de «limitar», según lo que necesites:
//
//   - channel con buffer N        → lo más simple y didáctico (este caso)
//
//   - errgroup.SetLimit(N)        → si ya usas errgroup (caso 1.2)
//
//   - semaphore.Weighted          → si cada tarea «pesa» distinto (p. ej. MB de memoria)
//
//     go run .
package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/semaphore"
)

// Monitor mide la concurrencia máxima observada.
type Monitor struct{ activas, Maximo atomic.Int32 }

func (m *Monitor) Entrar() {
	n := m.activas.Add(1)
	for {
		x := m.Maximo.Load()
		if n <= x || m.Maximo.CompareAndSwap(x, n) {
			return
		}
	}
}
func (m *Monitor) Salir() { m.activas.Add(-1) }

// ConChannel: el buffer de `sem` es el nº de plazas. Enviar = coger plaza (bloquea si no hay);
// recibir = soltarla. `struct{}` no ocupa memoria: solo importa el número de elementos.
func ConChannel(n, max int, trabajo func()) {
	sem := make(chan struct{}, max)
	var wg sync.WaitGroup
	for range n {
		sem <- struct{}{} // coger plaza ANTES de lanzar la goroutine: así no se lanzan n goroutines de golpe
		wg.Go(func() {
			defer func() { <-sem }() // soltar plaza (también si trabajo entra en pánico)
			trabajo()
		})
	}
	wg.Wait()
}

// ConPeso: semaphore.Weighted admite pesos distintos y es cancelable con ctx. Aquí, cada tarea declara
// cuántas «unidades» de un presupuesto total ocupa (p. ej. MB de RAM).
func ConPeso(ctx context.Context, presupuesto int64, pesos []int64, trabajo func(peso int64)) error {
	sem := semaphore.NewWeighted(presupuesto)
	var wg sync.WaitGroup
	for _, w := range pesos {
		if err := sem.Acquire(ctx, w); err != nil {
			wg.Wait()
			return err
		}
		wg.Go(func() {
			defer sem.Release(w)
			trabajo(w)
		})
	}
	wg.Wait()
	return nil
}

func main() {
	fmt.Println("20 tareas de 50 ms con distintos límites (tiempo ideal = ⌈20/límite⌉ × 50 ms):")
	for _, max := range []int{1, 2, 5, 10, 20} {
		var m Monitor
		t0 := time.Now()
		ConChannel(20, max, func() { m.Entrar(); time.Sleep(50 * time.Millisecond); m.Salir() })
		fmt.Printf("  límite %2d → %-6v máx. simultáneas observadas: %d\n", max, time.Since(t0).Round(10*time.Millisecond), m.Maximo.Load())
	}

	// Presupuesto de 100 MB: tareas de 60, 30, 50, 20, 40 MB. Nunca se superan los 100 MB en uso.
	var enUso, pico atomic.Int64
	pesos := []int64{60, 30, 50, 20, 40}
	ConPeso(context.Background(), 100, pesos, func(w int64) {
		n := enUso.Add(w)
		for {
			x := pico.Load()
			if n <= x || pico.CompareAndSwap(x, n) {
				break
			}
		}
		time.Sleep(30 * time.Millisecond)
		enUso.Add(-w)
	})
	fmt.Printf("con pesos %v y presupuesto 100: pico de memoria en uso = %d\n", pesos, pico.Load())
}
