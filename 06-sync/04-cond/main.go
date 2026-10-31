// Ejercicio 4: sync.Cond.
//
//  1. Cola acotada con dos Cond (noVacia / noLlena).
//  2. Broadcast como "pistoletazo de salida".
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Cola es una cola acotada bloqueante. Comparte UN mutex entre las dos Cond.
type Cola struct {
	mu      sync.Mutex
	noVacia *sync.Cond
	noLlena *sync.Cond
	datos   []int
	cap     int
}

func NuevaCola(cap int) *Cola {
	c := &Cola{cap: cap}
	c.noVacia = sync.NewCond(&c.mu)
	c.noLlena = sync.NewCond(&c.mu)
	return c
}

func (c *Cola) Poner(v int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for len(c.datos) == c.cap { // SIEMPRE for, nunca if: la condición se re-evalúa al despertar
		c.noLlena.Wait() // libera el mutex mientras espera y lo retoma al volver
	}
	c.datos = append(c.datos, v)
	c.noVacia.Signal() // despierta a UN consumidor
}

func (c *Cola) Sacar() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	for len(c.datos) == 0 {
		c.noVacia.Wait()
	}
	v := c.datos[0]
	c.datos = c.datos[1:]
	c.noLlena.Signal() // despierta a UN productor
	return v
}

func main() {
	// 1) Productores y consumidores sobre una cola de capacidad 3.
	cola := NuevaCola(3)
	const productores, consumidores, porProductor = 3, 2, 10
	var suma, recibidos atomic.Int64
	var wgP, wgC sync.WaitGroup

	for p := range productores {
		wgP.Go(func() {
			for i := 1; i <= porProductor; i++ {
				cola.Poner(p*100 + i)
			}
		})
	}
	total := productores * porProductor
	for range consumidores {
		wgC.Go(func() {
			for {
				if recibidos.Add(1) > int64(total) {
					return
				}
				suma.Add(int64(cola.Sacar()))
			}
		})
	}
	wgP.Wait()
	wgC.Wait()

	esperado := 0
	for p := range productores {
		for i := 1; i <= porProductor; i++ {
			esperado += p*100 + i
		}
	}
	fmt.Printf("cola: %d elementos consumidos, suma=%d (esperada %d)\n", total, suma.Load(), esperado)

	// 2) Broadcast: despierta a TODOS los que esperan una condición.
	var mu sync.Mutex
	cond := sync.NewCond(&mu)
	listo := false
	var wg sync.WaitGroup
	var arrancaron atomic.Int32

	for range 5 {
		wg.Go(func() {
			mu.Lock()
			for !listo {
				cond.Wait()
			}
			mu.Unlock()
			arrancaron.Add(1)
		})
	}
	time.Sleep(50 * time.Millisecond) // deja que todos se pongan a esperar
	fmt.Println("antes de Broadcast, arrancaron:", arrancaron.Load())

	mu.Lock()
	listo = true
	cond.Broadcast()
	mu.Unlock()
	wg.Wait()
	fmt.Println("tras Broadcast, arrancaron:", arrancaron.Load())
}
