// Ejercicio 2: fan-out / fan-in.
package main

import (
	"context"
	"fmt"
	"time"
)

func procesar(n int) int {
	time.Sleep(10 * time.Millisecond) // trabajo simulado
	return n * n
}

func ejecutar(workers int, entradas []int) (suma int, orden []int, d time.Duration) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ini := time.Now()
	in := Generar(ctx, entradas...)
	salidas := FanOut(ctx, in, workers, procesar) // fan-out: N workers sobre un mismo origen
	for v := range Merge(ctx, salidas...) {       // fan-in: un solo channel de salida
		suma += v
		orden = append(orden, v)
	}
	return suma, orden, time.Since(ini)
}

func main() {
	entradas := make([]int, 40)
	esperada := 0
	for i := range entradas {
		entradas[i] = i + 1
		esperada += (i + 1) * (i + 1)
	}

	fmt.Printf("suma de cuadrados esperada: %d\n\n", esperada)
	fmt.Printf("%8s %12s %10s\n", "workers", "tiempo", "suma ok")
	for _, w := range []int{1, 2, 4, 8} {
		suma, orden, d := ejecutar(w, entradas)
		fmt.Printf("%8d %12v %10v\n", w, d.Round(time.Millisecond), suma == esperada)
		if w == 4 {
			defer fmt.Println("\nprimeros 10 resultados con 4 workers (¡no están en orden!):", orden[:10])
		}
	}
}
