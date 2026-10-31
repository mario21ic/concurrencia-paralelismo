// Ejercicio 5: convertir entre channels e iteradores, y APIs paginadas perezosas.
package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync/atomic"
	"time"
)

// apiSimulada: 10 páginas de 3 elementos; cuenta cuántas páginas se pidieron.
func apiSimulada(paginasPedidas *atomic.Int32, falloEn int) func(context.Context, int) (Pagina[int], error) {
	return func(ctx context.Context, n int) (Pagina[int], error) {
		paginasPedidas.Add(1)
		select {
		case <-time.After(20 * time.Millisecond): // latencia de red
		case <-ctx.Done():
			return Pagina[int]{}, ctx.Err()
		}
		if n == falloEn {
			return Pagina[int]{}, errors.New("la API devolvió 503")
		}
		items := []int{n * 3, n*3 + 1, n*3 + 2}
		return Pagina[int]{Items: items, Siguiente: n < 9}, nil
	}
}

func main() {
	ctx := context.Background()

	// 1) Canal -> iterador.
	ch := make(chan string)
	go func() {
		defer close(ch)
		for _, s := range []string{"a", "b", "c"} {
			ch <- s
		}
	}()
	fmt.Println("DeCanal:", slices.Collect(DeCanal(ch)))

	// 2) Iterador -> canal (con cancelación).
	cctx, cancel := context.WithCancel(ctx)
	n := 0
	for v := range ACanal(cctx, slices.Values([]int{10, 20, 30, 40, 50})) {
		n += v
		if v == 30 {
			cancel() // detener al productor
			break
		}
	}
	cancel()
	fmt.Println("ACanal: suma hasta el 30 =", n)

	// 3) API paginada perezosa: tomar solo 5 elementos.
	var pedidas atomic.Int32
	var got []int
	for it, err := range Elementos(ctx, apiSimulada(&pedidas, -1)) {
		if err != nil {
			fmt.Println("error:", err)
			break
		}
		got = append(got, it)
		if len(got) == 5 {
			break
		}
	}
	fmt.Printf("\ntomé 5 elementos %v pidiendo solo %d de 10 páginas\n", got, pedidas.Load())

	// 4) Recorrerla entera.
	pedidas.Store(0)
	total := 0
	for _, err := range Elementos(ctx, apiSimulada(&pedidas, -1)) {
		if err == nil {
			total++
		}
	}
	fmt.Printf("recorrido completo: %d elementos, %d páginas pedidas\n", total, pedidas.Load())

	// 5) Error de la API a mitad.
	pedidas.Store(0)
	total = 0
	for _, err := range Elementos(ctx, apiSimulada(&pedidas, 2)) {
		if err != nil {
			fmt.Printf("\nerror tras %d elementos: %v\n", total, err)
			break
		}
		total++
	}

	// 6) Cancelación del contexto durante el recorrido.
	tctx, cancelT := context.WithTimeout(ctx, 70*time.Millisecond)
	defer cancelT()
	pedidas.Store(0)
	total = 0
	for _, err := range Elementos(tctx, apiSimulada(&pedidas, -1)) {
		if err != nil {
			fmt.Printf("con plazo de 70ms: %d elementos y error %q\n", total, err)
			break
		}
		total++
	}
}
