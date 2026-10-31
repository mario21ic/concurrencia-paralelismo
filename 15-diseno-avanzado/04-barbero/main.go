// Ejercicio 4: el barbero dormilón — capacidad, pérdidas y espera según barberos y sillas.
//
//	go run .
package main

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"
)

// simular lanza `n` clientes con llegadas exponenciales (media `llegada`) y cortes de `corte`.
func simular(barberos, sillas, n int, llegada, corte time.Duration) (perdidos int64, media, maxima time.Duration) {
	ctx, cancel := context.WithCancel(context.Background())
	b := Abrir(ctx, barberos, sillas, corte)
	rng := rand.New(rand.NewPCG(1, 2)) // misma secuencia en todas las configuraciones
	var wg sync.WaitGroup
	for range n {
		time.Sleep(time.Duration(rng.ExpFloat64() * float64(llegada)))
		wg.Go(func() { b.Llega() })
	}
	wg.Wait()
	cancel()
	b.Cerrar()
	media, maxima = b.Esperas()
	return b.Perdidos.Load(), media, maxima
}

func main() {
	const n = 300
	const llegada = 4 * time.Millisecond // media entre llegadas
	const corte = 6 * time.Millisecond
	fmt.Printf("%d clientes, llegada media %v, corte %v ⇒ un barbero tendría carga 1.5 (>1: no da abasto)\n\n", n, llegada, corte)
	fmt.Printf("  %-9s %-7s %-9s %-10s %-14s %s\n", "barberos", "sillas", "perdidos", "% perdido", "espera media", "espera máx")
	for _, c := range []struct{ barberos, sillas int }{
		{1, 0}, {1, 2}, {1, 10}, {2, 0}, {2, 2}, {2, 10}, {3, 2},
	} {
		p, media, maxima := simular(c.barberos, c.sillas, n, llegada, corte)
		fmt.Printf("  %-9d %-7d %-9d %-10.1f %-14v %v\n", c.barberos, c.sillas, p, 100*float64(p)/n,
			media.Round(100*time.Microsecond), maxima.Round(time.Millisecond))
	}
	fmt.Println("\nMás sillas = menos clientes perdidos pero más espera (latencia); más barberos = ambas mejoran.")
}
