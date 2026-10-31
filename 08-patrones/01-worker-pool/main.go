// Ejercicio 1: worker pool.
package main

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"
)

func main() {
	entradas := make([]int, 64)
	for i := range entradas {
		entradas[i] = i
	}

	// 1) Tiempo según el tamaño del pool (tarea I/O-bound de 20 ms).
	fmt.Println("== 64 tareas de 20ms ==")
	fmt.Printf("%8s %12s %16s\n", "workers", "tiempo", "máx. simultáneas")
	for _, w := range []int{1, 2, 4, 8, 16, 64} {
		var actuales, maximo atomic.Int32
		ini := time.Now()
		Procesar(context.Background(), w, entradas, func(ctx context.Context, n int) (int, error) {
			a := actuales.Add(1)
			for {
				m := maximo.Load()
				if a <= m || maximo.CompareAndSwap(m, a) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			actuales.Add(-1)
			return n * n, nil
		})
		fmt.Printf("%8d %12v %16d\n", w, time.Since(ini).Round(time.Millisecond), maximo.Load())
	}

	// 2) Orden preservado aunque las tareas terminen desordenadas.
	res := Procesar(context.Background(), 4, []int{5, 1, 4, 2, 3}, func(_ context.Context, n int) (string, error) {
		time.Sleep(time.Duration(6-n) * 10 * time.Millisecond) // las primeras tardan más
		return fmt.Sprint("r", n), nil
	})
	fmt.Print("\nresultados en orden de entrada: ")
	for _, r := range res {
		fmt.Print(r.Valor, " ")
	}
	fmt.Println()

	// 3) Errores por elemento + cancelación a mitad.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	res2 := Procesar(ctx, 4, entradas, func(_ context.Context, n int) (int, error) {
		time.Sleep(20 * time.Millisecond)
		if n%10 == 0 {
			return 0, fmt.Errorf("falló la tarea %d", n)
		}
		return n, nil
	})
	var ok, fallos, cancelados int
	for _, r := range res2 {
		switch {
		case r.Err == nil:
			ok++
		case errors.Is(r.Err, context.DeadlineExceeded):
			cancelados++
		default:
			fallos++
		}
	}
	fmt.Printf("\ncon plazo de 100ms: %d ok, %d con error propio, %d cancelados\n", ok, fallos, cancelados)
}
