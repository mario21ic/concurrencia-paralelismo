// Ejercicio 7: reproducir y arreglar un data race.
//
// 100 goroutines añaden su id a un slice compartido.
//
//	go run . -modo=bug           # pierde elementos
//	go run -race . -modo=bug     # el detector lo señala
//	go run . -modo=mutex|indice|canal
package main

import (
	"flag"
	"fmt"
	"sync"
)

const n = 1000

// bug: append no es seguro entre goroutines (lee len/cap, escribe y reasigna).
func bug() []int {
	var out []int
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() { out = append(out, i) })
	}
	wg.Wait()
	return out
}

// Solución 1: proteger con mutex.
func conMutex() []int {
	var (
		mu  sync.Mutex
		out []int
		wg  sync.WaitGroup
	)
	for i := range n {
		wg.Go(func() {
			mu.Lock()
			defer mu.Unlock()
			out = append(out, i)
		})
	}
	wg.Wait()
	return out
}

// Solución 2: slice preasignado, cada goroutine escribe en SU índice.
// Sin sincronización de datos: posiciones distintas, y wg.Wait() da el
// happens-before para leer luego.
func conIndice() []int {
	out := make([]int, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() { out[i] = i })
	}
	wg.Wait()
	return out
}

// Solución 3: las goroutines envían; una sola recolecta (dueña del slice).
func conCanal() []int {
	ch := make(chan int)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() { ch <- i })
	}
	go func() { wg.Wait(); close(ch) }()
	var out []int
	for v := range ch {
		out = append(out, v)
	}
	return out
}

func main() {
	modo := flag.String("modo", "bug", "bug|mutex|indice|canal")
	flag.Parse()
	fn := map[string]func() []int{"bug": bug, "mutex": conMutex, "indice": conIndice, "canal": conCanal}[*modo]
	if fn == nil {
		flag.Usage()
		return
	}
	r := fn()
	fmt.Printf("%s: %d elementos (esperados %d)\n", *modo, len(r), n)
}
