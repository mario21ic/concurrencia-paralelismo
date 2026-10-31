// Ejercicio 2: captura de variables de bucle.
//
// Desde Go 1.22 cada iteración de un `for` tiene su PROPIA variable.
// Aquí se compara con el comportamiento antiguo, reproducido declarando
// la variable fuera del bucle (compartida entre iteraciones).
package main

import (
	"fmt"
	"slices"
	"sync"
)

func recolectar(lanzar func(emit func(int), wg *sync.WaitGroup)) []int {
	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		out []int
	)
	emit := func(v int) {
		mu.Lock()
		out = append(out, v)
		mu.Unlock()
	}
	lanzar(emit, &wg)
	wg.Wait()
	slices.Sort(out)
	return out
}

func main() {
	// Go 1.22+: i es nueva en cada iteración.
	nuevo := recolectar(func(emit func(int), wg *sync.WaitGroup) {
		for i := 0; i < 5; i++ {
			wg.Go(func() { emit(i) })
		}
	})
	fmt.Println("semántica 1.22 (i por iteración):", nuevo)

	// Comportamiento antiguo: una única i compartida (y con data race).
	// Las goroutines suelen ver el valor final (5) o uno avanzado.
	var i int
	viejo := recolectar(func(emit func(int), wg *sync.WaitGroup) {
		for i = 0; i < 5; i++ {
			wg.Go(func() { emit(i) }) // lee i mientras el bucle la modifica
		}
	})
	fmt.Println("variable compartida (estilo <1.22):", viejo)

	// Solución clásica pre-1.22, aún válida: pasar como argumento.
	arg := recolectar(func(emit func(int), wg *sync.WaitGroup) {
		var j int
		for j = 0; j < 5; j++ {
			wg.Go(func(v int) func() { return func() { emit(v) } }(j))
		}
	})
	fmt.Println("pasando el valor como argumento:", arg)
}
