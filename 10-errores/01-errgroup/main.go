// Ejercicio 1: golang.org/x/sync/errgroup.
package main

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

// tarea espera d y devuelve un error si fallar es true. Respeta ctx.
func tarea(ctx context.Context, id int, d time.Duration, fallar bool) error {
	select {
	case <-time.After(d):
		if fallar {
			return fmt.Errorf("tarea %d falló", id)
		}
		return nil
	case <-ctx.Done():
		return fmt.Errorf("tarea %d cancelada: %w", id, ctx.Err())
	}
}

func main() {
	// 1) Sin WithContext: Wait devuelve el PRIMER error, pero no cancela a nadie.
	var g errgroup.Group
	var terminadas atomic.Int32
	ini := time.Now()
	for i := 1; i <= 4; i++ {
		g.Go(func() error {
			err := tarea(context.Background(), i, time.Duration(i)*50*time.Millisecond, i == 1)
			terminadas.Add(1)
			return err
		})
	}
	err := g.Wait()
	fmt.Printf("1) sin contexto: err=%q, terminaron %d/4, tiempo=%v (esperó a todas)\n", err, terminadas.Load(), time.Since(ini).Round(10*time.Millisecond))

	// 2) Con WithContext: el primer error CANCELA el contexto derivado.
	g2, ctx := errgroup.WithContext(context.Background())
	ini = time.Now()
	for i := 1; i <= 4; i++ {
		g2.Go(func() error {
			return tarea(ctx, i, time.Duration(i)*50*time.Millisecond, i == 1)
		})
	}
	err = g2.Wait()
	fmt.Printf("2) con contexto: err=%q, tiempo=%v (las demás se cancelaron al instante)\n", err, time.Since(ini).Round(10*time.Millisecond))
	fmt.Println("   ctx tras Wait:", ctx.Err(), "(Wait siempre cancela el contexto derivado)")

	// 3) Error típico: usar el ctx EXTERNO en lugar del derivado.
	g3, _ := errgroup.WithContext(context.Background())
	ini = time.Now()
	externo := context.Background()
	for i := 1; i <= 4; i++ {
		g3.Go(func() error {
			return tarea(externo, i, time.Duration(i)*50*time.Millisecond, i == 1) // ¡no ve la cancelación!
		})
	}
	g3.Wait()
	fmt.Printf("3) BUG (ctx externo): tiempo=%v (nadie se enteró del error)\n", time.Since(ini).Round(10*time.Millisecond))

	// 4) SetLimit: como mucho N goroutines a la vez; Go() BLOQUEA si está lleno.
	var g4 errgroup.Group
	g4.SetLimit(3)
	var actuales, maximo atomic.Int32
	ini = time.Now()
	for range 10 {
		g4.Go(func() error {
			a := actuales.Add(1)
			for m := maximo.Load(); a > m && !maximo.CompareAndSwap(m, a); m = maximo.Load() {
			}
			time.Sleep(30 * time.Millisecond)
			actuales.Add(-1)
			return nil
		})
	}
	g4.Wait()
	fmt.Printf("4) SetLimit(3): 10 tareas de 30ms -> máx. simultáneas=%d, tiempo=%v (ideal 120ms)\n", maximo.Load(), time.Since(ini).Round(10*time.Millisecond))

	// 5) TryGo: no bloquea; devuelve false si no hay hueco.
	var g5 errgroup.Group
	g5.SetLimit(1)
	parar := make(chan struct{})
	fmt.Println("5) TryGo con hueco:", g5.TryGo(func() error { <-parar; return nil }))
	fmt.Println("   TryGo sin hueco:", g5.TryGo(func() error { return nil }))
	close(parar)
	g5.Wait()

	// 6) errgroup solo conserva el PRIMER error; los demás se pierden.
	var g6 errgroup.Group
	for i := 1; i <= 3; i++ {
		g6.Go(func() error { time.Sleep(time.Duration(i) * 10 * time.Millisecond); return fmt.Errorf("fallo %d", i) })
	}
	err = g6.Wait()
	fmt.Println("6) tres tareas fallaron; Wait devuelve solo el primero:", err, "(los otros dos errores se perdieron)")
}
