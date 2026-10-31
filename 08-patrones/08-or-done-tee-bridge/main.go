// Ejercicio 8: or-done, tee y bridge.
package main

import (
	"context"
	"fmt"
	"sync"
)

func generar(ctx context.Context, vals ...int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for _, v := range vals {
			select {
			case out <- v:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1) OrDone: leer con range y poder cancelar sin select en cada lectura.
	fmt.Print("OrDone: ")
	for v := range OrDone(ctx, generar(ctx, 1, 2, 3)) {
		fmt.Print(v, " ")
	}
	fmt.Println()

	// 2) Tee: dos consumidores independientes ven el MISMO flujo.
	a, b := Tee(ctx, generar(ctx, 10, 20, 30))
	var wg sync.WaitGroup
	var ra, rb []int
	wg.Go(func() {
		for v := range a {
			ra = append(ra, v)
		}
	})
	wg.Go(func() {
		for v := range b {
			rb = append(rb, v*2) // cada consumidor hace lo suyo
		}
	})
	wg.Wait()
	fmt.Println("Tee: consumidor A =", ra, "| consumidor B (doble) =", rb)

	// 3) Bridge: un channel de channels (p. ej. un flujo por lote) -> un solo flujo.
	flujo := make(chan (<-chan int))
	go func() {
		defer close(flujo)
		for lote := range 3 {
			flujo <- generar(ctx, lote*10, lote*10+1, lote*10+2)
		}
	}()
	fmt.Print("Bridge: ")
	for v := range Bridge(ctx, flujo) {
		fmt.Print(v, " ")
	}
	fmt.Println()
}
