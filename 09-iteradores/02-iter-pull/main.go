// Ejercicio 2: iter.Pull — pasar de "push" (range) a "pull" (next/stop).
package main

import (
	"fmt"
	"iter"
	"runtime"
	"slices"
	"time"
)

// Mezclar combina dos secuencias ORDENADAS en una. Con range solo se puede
// recorrer una a la vez; para avanzar dos "en paralelo" hace falta Pull.
func Mezclar[T any](a, b iter.Seq[T], menor func(x, y T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		nextA, stopA := iter.Pull(a)
		defer stopA()
		nextB, stopB := iter.Pull(b)
		defer stopB()

		va, okA := nextA()
		vb, okB := nextB()
		for okA && okB {
			if menor(va, vb) {
				if !yield(va) {
					return
				}
				va, okA = nextA()
			} else {
				if !yield(vb) {
					return
				}
				vb, okB = nextB()
			}
		}
		for ; okA; va, okA = nextA() {
			if !yield(va) {
				return
			}
		}
		for ; okB; vb, okB = nextB() {
			if !yield(vb) {
				return
			}
		}
	}
}

// Zip empareja dos secuencias elemento a elemento; termina con la más corta.
func Zip[A, B any](a iter.Seq[A], b iter.Seq[B]) iter.Seq2[A, B] {
	return func(yield func(A, B) bool) {
		nextA, stopA := iter.Pull(a)
		defer stopA()
		nextB, stopB := iter.Pull(b)
		defer stopB()
		for {
			va, okA := nextA()
			vb, okB := nextB()
			if !okA || !okB || !yield(va, vb) {
				return
			}
		}
	}
}

func menor(x, y int) bool { return x < y }

func main() {
	// 1) Uso básico: next devuelve (valor, ok); stop libera recursos.
	next, stop := iter.Pull(slices.Values([]string{"a", "b", "c"}))
	for {
		v, ok := next()
		if !ok {
			break
		}
		fmt.Print(v, " ")
	}
	stop() // idempotente; tras el fin natural no hace falta, pero es buena costumbre (defer stop())
	fmt.Println("<- Pull básico")

	// 2) Mezcla de dos secuencias ordenadas.
	a := slices.Values([]int{1, 4, 7, 10})
	b := slices.Values([]int{2, 3, 8, 9, 11, 12})
	fmt.Println("mezcla ordenada:", slices.Collect(Mezclar(a, b, menor)))

	// 3) Zip.
	fmt.Print("zip: ")
	for x, s := range Zip(slices.Values([]int{1, 2, 3}), slices.Values([]string{"uno", "dos"})) {
		fmt.Printf("(%d,%s) ", x, s)
	}
	fmt.Println()

	// 4) Olvidar stop() deja el iterador "congelado" a mitad: fuga.
	base := runtime.NumGoroutine()
	for range 100 {
		next, _ := iter.Pull(slices.Values([]int{1, 2, 3}))
		next() // se consume un elemento y se abandona SIN stop()
	}
	runtime.GC()
	time.Sleep(20 * time.Millisecond)
	fmt.Println("\ngoroutines (corrutinas) fugadas por olvidar stop():", runtime.NumGoroutine()-base)

	base = runtime.NumGoroutine()
	for range 100 {
		next, stop := iter.Pull(slices.Values([]int{1, 2, 3}))
		next()
		stop() // correcto
	}
	time.Sleep(20 * time.Millisecond)
	fmt.Println("goroutines fugadas llamando a stop():            ", runtime.NumGoroutine()-base)

	// 5) Coste de un next(): cambio de corrutina, mucho más barato que un channel.
	const n = 1_000_000
	nx, st := iter.Pull(func(yield func(int) bool) {
		for i := 0; ; i++ {
			if !yield(i) {
				return
			}
		}
	})
	ini := time.Now()
	for range n {
		nx()
	}
	d := time.Since(ini)
	st()
	fmt.Printf("\n%d llamadas a next(): %v (%v por llamada)\n", n, d.Round(time.Millisecond), d/n)

	ch := make(chan int)
	go func() {
		for i := 0; ; i++ {
			ch <- i
		}
	}()
	ini = time.Now()
	for range n {
		<-ch
	}
	d = time.Since(ini)
	fmt.Printf("%d recepciones de channel sin buffer: %v (%v por recepción)\n", n, d.Round(time.Millisecond), d/n)
}
