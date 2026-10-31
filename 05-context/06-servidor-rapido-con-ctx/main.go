// Ejercicio 6: "el servidor más rápido" (módulo 4) con cancelación REAL.
//
// Al ganar uno, se cancela el contexto y los demás dejan de trabajar
// inmediatamente, en lugar de seguir hasta el final.
package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

type servidor struct {
	nombre  string
	latency time.Duration
}

type traza struct {
	nombre    string
	duracion  time.Duration
	cancelado bool
}

func consultar(ctx context.Context, s servidor) (string, error) {
	select {
	case <-time.After(s.latency):
		return "datos de " + s.nombre, nil
	case <-ctx.Done():
		return "", context.Cause(ctx)
	}
}

// primero devuelve la primera respuesta. Si cancelar es true, cancela al resto.
func primero(ctx context.Context, servidores []servidor, cancelar bool) (string, []traza) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	ch := make(chan string, len(servidores)) // buffer: nadie se bloquea al enviar
	trazas := make([]traza, len(servidores))
	var wg sync.WaitGroup
	// sin cancelar, los servidores usan un contexto que nunca termina
	trabajo := ctx
	if !cancelar {
		trabajo = context.Background()
	}

	for i, s := range servidores {
		wg.Go(func() {
			ini := time.Now()
			dato, err := consultar(trabajo, s)
			trazas[i] = traza{s.nombre, time.Since(ini), err != nil}
			if err == nil {
				ch <- dato
			}
		})
	}
	ganador := <-ch
	cancel() // con cancelar=true los demás paran aquí
	wg.Wait()
	return ganador, trazas
}

func main() {
	servidores := []servidor{
		{"eu-oeste", 300 * time.Millisecond},
		{"us-este", 100 * time.Millisecond},
		{"asia", 500 * time.Millisecond},
	}

	for _, cancelar := range []bool{false, true} {
		ini := time.Now()
		g, trazas := primero(context.Background(), servidores, cancelar)
		fmt.Printf("\ncancelar=%v: ganó %q; todas las goroutines terminaron a los %v\n", cancelar, g, time.Since(ini).Round(10*time.Millisecond))
		for _, t := range trazas {
			estado := "completó"
			if t.cancelado {
				estado = "CANCELADO"
			}
			fmt.Printf("  %-9s trabajó %-6v %s\n", t.nombre, t.duracion.Round(10*time.Millisecond), estado)
		}
	}
}
