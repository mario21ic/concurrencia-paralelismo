// Ejercicio 4: And / Or atómicos (Go 1.23+) — banderas en una máscara de bits.
package main

import (
	"fmt"
	"math/bits"
	"sync"
	"sync/atomic"
)

const (
	Leer uint32 = 1 << iota
	Escribir
	Ejecutar
)

func main() {
	var perms atomic.Uint32

	// Or: activa bits. Devuelve el valor ANTERIOR.
	ant := perms.Or(Leer | Escribir)
	fmt.Printf("Or(Leer|Escribir)  anterior=%03b ahora=%03b\n", ant, perms.Load())

	// And con la máscara complementaria: apaga bits.
	ant = perms.And(^Escribir)
	fmt.Printf("And(^Escribir)     anterior=%03b ahora=%03b\n", ant, perms.Load())

	// Detectar si YO fui quien activó el bit (Or devuelve el valor previo).
	ant = perms.Or(Ejecutar)
	fmt.Println("¿activé Ejecutar yo?      ", ant&Ejecutar == 0)
	ant = perms.Or(Ejecutar)
	fmt.Println("¿lo activé de nuevo yo?   ", ant&Ejecutar == 0, "(ya estaba activo)")

	// Forma con funciones para variables simples.
	var x uint32
	atomic.OrUint32(&x, 0b1010)
	atomic.AndUint32(&x, 0b0110)
	fmt.Printf("OrUint32/AndUint32: %04b\n", x)

	// 32 goroutines activan cada una SU bit. Se repite 500 veces y se cuentan
	// las rondas en las que se perdió algún bit.
	const n, rondas = 32, 500
	var wg sync.WaitGroup
	var rondasMalas, rondasBuenas, bitsPerdidos int

	for range rondas {
		// MAL: Load + Store (cada operación es atómica, la secuencia no).
		var mal atomic.Uint32
		for i := range n {
			wg.Go(func() {
				v := mal.Load()
				mal.Store(v | 1<<i) // otra goroutine pudo cambiar `mal` entre Load y Store
			})
		}
		wg.Wait()
		if c := bits.OnesCount32(mal.Load()); c != n {
			rondasMalas++
			bitsPerdidos += n - c
		}

		// BIEN: Or atómico.
		var bien atomic.Uint32
		for i := range n {
			wg.Go(func() { bien.Or(1 << i) })
		}
		wg.Wait()
		if bits.OnesCount32(bien.Load()) != n {
			rondasBuenas++
		}
	}
	fmt.Printf("\nLoad+Store: %d/%d rondas perdieron bits (%d bits perdidos en total)\n", rondasMalas, rondas, bitsPerdidos)
	fmt.Printf("Or atómico: %d/%d rondas con bits perdidos\n", rondasBuenas, rondas)
}
