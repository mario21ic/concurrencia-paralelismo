// Ejercicio 1: iteradores (iter.Seq / iter.Seq2) y range-over-func.
package main

import (
	"fmt"
	"iter"
	"maps"
	"slices"
	"strings"
)

func par(n int) bool { return n%2 == 0 }

// conLimpieza muestra que un `defer` dentro del iterador se ejecuta SIEMPRE,
// incluso si el consumidor hace break: es la forma de liberar recursos.
func conLimpieza() iter.Seq[int] {
	return func(yield func(int) bool) {
		fmt.Println("  [iterador] abriendo recurso")
		defer fmt.Println("  [iterador] cerrando recurso")
		for i := 1; i <= 5; i++ {
			if !yield(i) {
				return
			}
		}
	}
}

// malo ignora el false de yield: viola el contrato.
func malo() iter.Seq[int] {
	return func(yield func(int) bool) {
		for i := 1; i <= 5; i++ {
			yield(i) // ¡no comprueba el resultado!
		}
	}
}

func main() {
	// 1) Composición perezosa: nada se calcula hasta que el range pide valores.
	fmt.Print("primeros 5 cuadrados de pares: ")
	for v := range Take(Map(Filter(Contar(1), par), func(n int) int { return n * n }), 5) {
		fmt.Print(v, " ")
	}
	fmt.Println()

	fmt.Print("Fibonacci < 100: ")
	for f := range Fib() {
		if f >= 100 {
			break // la secuencia es infinita; el break basta
		}
		fmt.Print(f, " ")
	}
	fmt.Println()

	// 2) Seq2: pares (clave, valor) o (índice, valor).
	for i, v := range Enumerar(Take(Fib(), 5)) {
		fmt.Printf("fib[%d]=%d ", i, v)
	}
	fmt.Println()

	// 3) Biblioteca estándar: slices, maps y strings devuelven iteradores.
	m := map[string]int{"b": 2, "a": 1, "c": 3}
	fmt.Println("claves ordenadas:", slices.Sorted(maps.Keys(m)))
	fmt.Println("valores sumados: ", func() (s int) {
		for v := range maps.Values(m) {
			s += v
		}
		return
	}())
	for lote := range slices.Chunk([]int{1, 2, 3, 4, 5, 6, 7}, 3) {
		fmt.Print(lote, " ")
	}
	fmt.Println("<- slices.Chunk")
	for parte := range strings.SplitSeq("a,b,c", ",") { // Go 1.24+: sin crear el slice intermedio
		fmt.Print(parte, " ")
	}
	fmt.Println("<- strings.SplitSeq")
	fmt.Println("slices.Collect:", slices.Collect(Take(Contar(10), 4)))

	// 4) Limpieza con defer al hacer break.
	fmt.Println("\nbreak dentro del bucle:")
	for v := range conLimpieza() {
		fmt.Println("  consumidor recibe", v)
		if v == 2 {
			break
		}
	}

	// 5) Un iterador que ignora false produce un panic en tiempo de ejecución.
	func() {
		defer func() { fmt.Println("\niterador malo ->", recover()) }()
		for v := range malo() {
			if v == 2 {
				break
			}
		}
	}()
}
