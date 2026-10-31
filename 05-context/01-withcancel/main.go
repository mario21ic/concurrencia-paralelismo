// Ejercicio 1: context.WithCancel y el árbol de contextos.
//
//	raíz (Background)
//	 └─ padre (WithCancel)
//	     ├─ hijoA (WithCancel)
//	     └─ hijoB (WithCancel)
//
// Cancelar el padre cancela a los hijos; cancelar un hijo NO afecta al padre.
package main

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"time"
)

func escuchar(ctx context.Context, nombre string, wg *sync.WaitGroup, out chan<- string) {
	wg.Go(func() {
		<-ctx.Done()
		out <- fmt.Sprintf("%s cancelado: %v", nombre, ctx.Err())
	})
}

func main() {
	padre, cancelPadre := context.WithCancel(context.Background())
	hijoA, cancelA := context.WithCancel(padre)
	hijoB, cancelB := context.WithCancel(padre)
	defer cancelB() // siempre llamar a cancel: libera recursos

	fmt.Println("antes: padre.Err =", padre.Err(), "| hijoA.Err =", hijoA.Err())

	// 1) Cancelar un hijo no afecta al padre ni al hermano.
	cancelA()
	fmt.Println("tras cancelar hijoA: hijoA.Err =", hijoA.Err(), "| padre.Err =", padre.Err(), "| hijoB.Err =", hijoB.Err())

	// 2) Cancelar el padre cancela a todos los descendientes.
	var wg sync.WaitGroup
	out := make(chan string, 3)
	escuchar(padre, "padre", &wg, out)
	escuchar(hijoA, "hijoA", &wg, out)
	escuchar(hijoB, "hijoB", &wg, out)

	time.Sleep(20 * time.Millisecond)
	cancelPadre()
	wg.Wait()
	close(out)

	var lineas []string
	for l := range out {
		lineas = append(lineas, l)
	}
	slices.Sort(lineas)
	for _, l := range lineas {
		fmt.Println(l)
	}

	// 3) Cancelar es idempotente; Done queda cerrado para siempre.
	cancelPadre()
	cancelPadre()
	_, abierto := <-padre.Done()
	fmt.Println("padre.Done() sigue cerrado:", !abierto)
}
