// Ejercicio 5: sync.Map — API y casos de uso.
// Los benchmarks están en bench_test.go: go test -bench . -benchmem
package main

import (
	"fmt"
	"slices"
	"sync"
)

func main() {
	var m sync.Map // el valor cero está listo para usar; no se copia

	m.Store("a", 1)
	m.Store("b", 2)

	v, ok := m.Load("a")
	fmt.Println("Load a:", v, ok) // los valores son `any`: hay que hacer type assertion
	_, ok = m.Load("z")
	fmt.Println("Load z:", ok)

	// LoadOrStore: atómico "si no existe, guarda". Devuelve loaded=true si ya existía.
	act, loaded := m.LoadOrStore("c", 3)
	fmt.Println("LoadOrStore c (nuevo):", act, loaded)
	act, loaded = m.LoadOrStore("c", 99)
	fmt.Println("LoadOrStore c (existente):", act, loaded)

	// Swap, CompareAndSwap, CompareAndDelete.
	prev, _ := m.Swap("a", 10)
	fmt.Println("Swap a: antes =", prev)
	fmt.Println("CompareAndSwap a 10->11:", m.CompareAndSwap("a", 10, 11))
	fmt.Println("CompareAndSwap a 10->12:", m.CompareAndSwap("a", 10, 12), "(ya no vale 10)")

	// LoadAndDelete: borra y devuelve lo que había.
	v, loaded = m.LoadAndDelete("b")
	fmt.Println("LoadAndDelete b:", v, loaded)

	// Range: no es un snapshot consistente; admite modificar durante el recorrido.
	var claves []string
	m.Range(func(k, v any) bool {
		claves = append(claves, fmt.Sprintf("%v=%v", k, v))
		return true // false detiene el recorrido
	})
	slices.Sort(claves)
	fmt.Println("Range:", claves)

	// Clear (Go 1.23+): vacía el mapa.
	m.Clear()
	n := 0
	m.Range(func(_, _ any) bool { n++; return true })
	fmt.Println("tras Clear, elementos:", n)

	// Caso de uso típico: "cachear una vez por clave" con concurrencia.
	var wg sync.WaitGroup
	var cache sync.Map
	var calculos sync.Map
	for range 20 {
		wg.Go(func() {
			_, cargado := cache.LoadOrStore("pi", 3.14159)
			if !cargado {
				calculos.Store("quien-guardó", true)
			}
		})
	}
	wg.Wait()
	_, guardó := calculos.Load("quien-guardó")
	fmt.Println("20 goroutines, el valor se guardó una sola vez:", guardó)
}
