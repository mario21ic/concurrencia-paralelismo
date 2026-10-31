// Ejercicio 3: la cancelación es COOPERATIVA.
//
// cancel() solo cierra un channel. Nada detiene una goroutine a la fuerza:
// debe revisar ctx.Done()/ctx.Err() ella misma.
package main

import (
	"context"
	"fmt"
	"time"
)

// ignora: nunca mira ctx. Tarda lo mismo con o sin cancelación.
func ignora(ctx context.Context) {
	time.Sleep(500 * time.Millisecond)
}

// sleepCancelable sustituye a time.Sleep respetando el contexto.
func sleepCancelable(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

// bucleBloqueante: espera en select para reaccionar al instante.
func bucleBloqueante(ctx context.Context) int {
	n := 0
	for {
		if err := sleepCancelable(ctx, 50*time.Millisecond); err != nil {
			return n
		}
		n++
	}
}

// bucleCPU: cálculo sin esperas; comprueba ctx.Err() cada cierto número de
// iteraciones (consultarlo en cada vuelta sería caro).
func bucleCPU(ctx context.Context) (iteraciones int) {
	for {
		if iteraciones%10_000 == 0 && ctx.Err() != nil {
			return
		}
		iteraciones++
	}
}

func medir(nombre string, f func(ctx context.Context)) {
	ctx, cancel := context.WithCancel(context.Background())
	hecho := make(chan struct{})
	go func() {
		defer close(hecho)
		f(ctx)
	}()
	time.Sleep(100 * time.Millisecond)
	ini := time.Now()
	cancel()
	<-hecho
	fmt.Printf("%-28s tardó en parar: %v\n", nombre, time.Since(ini).Round(time.Millisecond))
}

func main() {
	medir("ignora el contexto:", ignora)
	medir("select con ctx.Done():", func(ctx context.Context) { bucleBloqueante(ctx) })
	medir("bucle CPU con ctx.Err():", func(ctx context.Context) { bucleCPU(ctx) })
}
