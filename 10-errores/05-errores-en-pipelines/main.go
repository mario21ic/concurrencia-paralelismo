// Ejercicio 5: errores en pipelines con errgroup.
package main

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"
)

func main() {
	ctx := context.Background()
	base := runtime.NumGoroutine()

	lineas := make([]string, 10_000)
	esperado := 0
	for i := range lineas {
		lineas[i] = strconv.Itoa(i)
		esperado += i
	}

	// 1) Entrada válida.
	var proc atomic.Int64
	total, err := SumarLineas(ctx, lineas, 4, &proc)
	fmt.Printf("1) válida:   total=%d (esperado %d) err=%v procesadas=%d\n", total, esperado, err, proc.Load())

	// 2) Una línea inválida en medio: el pipeline se detiene.
	malas := append([]string(nil), lineas...)
	malas[5000] = "5000x"
	proc.Store(0)
	total, err = SumarLineas(ctx, malas, 4, &proc)
	fmt.Printf("2) inválida: total=%d err=%v\n", total, err)
	fmt.Printf("   procesadas %d de %d líneas antes de abortar (el resto se evitó)\n", proc.Load(), len(malas))

	// 3) Cancelación externa mientras el pipeline trabaja.
	cctx, cancel := context.WithCancel(ctx)
	var proc3 atomic.Int64
	go func() {
		for proc3.Load() < 100 { // cancelar cuando ya hay trabajo en marcha
			time.Sleep(10 * time.Microsecond)
		}
		cancel()
	}()
	_, err = SumarLineas(cctx, lineas, 4, &proc3)
	fmt.Printf("3) cancelación externa tras ~100 líneas: err=%v, procesadas=%d de %d\n", err, proc3.Load(), len(lineas))

	time.Sleep(50 * time.Millisecond)
	fmt.Println("\ngoroutines fugadas:", runtime.NumGoroutine()-base)
}
