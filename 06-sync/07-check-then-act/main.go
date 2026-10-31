// Ejercicio 7: componer operaciones seguras NO es seguro.
//
// Cada operación del conjunto está protegida por un mutex, pero la secuencia
// "comprobar y luego actuar" no es atómica.
//
//	go run .
//	go run -race .
package main

import (
	"fmt"
	"runtime"
	"sync"
)

// ConjuntoMal: cada método es seguro por separado...
type ConjuntoMal struct {
	mu sync.Mutex
	m  map[int]bool
}

func (c *ConjuntoMal) Contiene(k int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.m[k]
}

func (c *ConjuntoMal) Agregar(k int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[k] = true
}

// ConjuntoBien: la operación compuesta es UNA sola sección crítica.
type ConjuntoBien struct {
	mu sync.Mutex
	m  map[int]bool
}

// AgregarSiNoExiste devuelve true si fue el que insertó.
func (c *ConjuntoBien) AgregarSiNoExiste(k int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m[k] {
		return false
	}
	c.m[k] = true
	return true
}

// ronda lanza goroutines que intentan ser "las primeras" en insertar la clave 42
// y devuelve cuántas creyeron serlo.
func ronda(goroutines int, insertar func() bool) int {
	var (
		wg     sync.WaitGroup
		mu     sync.Mutex
		primer int
	)
	inicio := make(chan struct{}) // barrera: todas arrancan a la vez para forzar la carrera
	for range goroutines {
		wg.Go(func() {
			<-inicio
			if insertar() {
				mu.Lock()
				primer++
				mu.Unlock()
			}
		})
	}
	close(inicio)
	wg.Wait()
	return primer
}

func main() {
	const goroutines, rondas = 100, 500

	var rondasMalas, peorMal, rondasMalasBien int
	for range rondas {
		mal := &ConjuntoMal{m: map[int]bool{}}
		n := ronda(goroutines, func() bool {
			if !mal.Contiene(42) { // <- aquí otro puede colarse
				runtime.Gosched() // amplifica la ventana (en código real basta la mala suerte)
				mal.Agregar(42)
				return true
			}
			return false
		})
		if n != 1 {
			rondasMalas++
		}
		peorMal = max(peorMal, n)

		bien := &ConjuntoBien{m: map[int]bool{}}
		if ronda(goroutines, func() bool { return bien.AgregarSiNoExiste(42) }) != 1 {
			rondasMalasBien++
		}
	}
	fmt.Printf("check-then-act:    %d/%d rondas con más de un \"primero\" (peor ronda: %d a la vez)\n", rondasMalas, rondas, peorMal)
	fmt.Printf("operación atómica: %d/%d rondas incorrectas\n", rondasMalasBien, rondas)
}
