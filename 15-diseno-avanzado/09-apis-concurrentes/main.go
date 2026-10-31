// Ejercicio 9: diseño de APIs concurrentes y concurrencia estructurada.
//
//	go run .
package main

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"
)

func goroutines() int { time.Sleep(20 * time.Millisecond); return runtime.NumGoroutine() }

func main() {
	base := goroutines()
	fmt.Printf("goroutines al empezar: %d\n", base)

	fmt.Println("\n== 1) ¿quién lanza la goroutine? ==")
	for range 100 {
		NuevoRelojMal(time.Hour)
	}
	fmt.Printf("  100 × NuevoRelojMal()  → +%d goroutines (sin forma de pararlas)\n", goroutines()-base)

	base = goroutines()
	ctx, cancel := context.WithCancel(context.Background())
	for range 100 {
		r := NuevoReloj(time.Hour)
		go r.Run(ctx)
	}
	fmt.Printf("  100 × go Reloj.Run(ctx) → +%d goroutines…", goroutines()-base)
	cancel()
	fmt.Printf(" tras cancelar(ctx): +%d\n", goroutines()-base)

	fmt.Println("\n== 2) API de flujo con un consumidor que abandona a mitad ==")
	base = goroutines()
	for range 100 {
		out := DuplicarMal([]int{1, 2, 3})
		<-out
	}
	fmt.Printf("  DuplicarMal ×100 (lee 1 de 3)      → +%d goroutines bloqueadas para siempre\n", goroutines()-base)

	base = goroutines()
	for range 100 {
		c, cancel := context.WithCancel(context.Background())
		out := Duplicar(c, []int{1, 2, 3})
		<-out
		cancel()
	}
	fmt.Printf("  Duplicar(ctx) ×100 (lee 1 de 3)    → +%d\n", goroutines()-base)

	base = goroutines()
	for range 100 {
		for v := range DuplicarIter([]int{1, 2, 3}) {
			_ = v
			break
		}
	}
	fmt.Printf("  DuplicarIter ×100 (break al primero) → +%d (iterador síncrono: no hay goroutine)\n", goroutines()-base)

	fmt.Println("\n== 3) concurrencia estructurada: 1 URL falla a los 20 ms, las otras 9 tardan 500 ms ==")
	fetch := func(ctx context.Context, u string) (string, error) {
		if u == "mala" {
			time.Sleep(20 * time.Millisecond)
			return "", errors.New("503")
		}
		select {
		case <-time.After(500 * time.Millisecond):
			return u, nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	urls := []string{"mala", "a", "b", "c", "d", "e", "f", "g", "h", "i"}

	base = goroutines()
	t0 := time.Now()
	_, err := DescargarTodas(context.Background(), urls, 10, fetch)
	fmt.Printf("  DescargarTodas : volvió a los %v con error %q; goroutines vivas tras volver: +%d\n",
		time.Since(t0).Round(time.Millisecond), err, runtime.NumGoroutine()-base)

	t0 = time.Now()
	_, err = DescargarMal(urls, func(_ context.Context, u string) (string, error) { return fetch(context.Background(), u) })
	fmt.Printf("  DescargarMal   : volvió a los %v con error %q; goroutines vivas tras volver: +%d (siguen 480 ms más)\n",
		time.Since(t0).Round(time.Millisecond), err, runtime.NumGoroutine()-base)
	time.Sleep(600 * time.Millisecond)
}
