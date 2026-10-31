// Ejercicio 1: operaciones atómicas básicas y tipos atómicos.
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

const (
	goroutines  = 100
	incrementos = 10_000
)

func main() {
	// 1) Con atomic.Int64 el resultado es exacto. La versión SIN atomic (que
	//    pierde incrementos) está en el módulo 1, ejercicio 6.
	var bueno atomic.Int64
	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			for range incrementos {
				bueno.Add(1)
			}
		})
	}
	wg.Wait()
	fmt.Printf("atomic.Int64: %d (esperado %d)\n", bueno.Load(), goroutines*incrementos)

	// 2) Valores de retorno de cada operación.
	var x atomic.Int64
	fmt.Println("\nAdd(5)  devuelve el NUEVO valor:    ", x.Add(5))
	fmt.Println("Swap(9) devuelve el valor ANTERIOR: ", x.Swap(9))
	fmt.Println("CompareAndSwap(9, 20) =>            ", x.CompareAndSwap(9, 20), "(valor ahora", x.Load(), ")")
	fmt.Println("CompareAndSwap(9, 30) =>            ", x.CompareAndSwap(9, 30), "(ya no era 9; sigue", x.Load(), ")")
	x.Store(100)
	fmt.Println("Store(100); Load() =                ", x.Load())

	// 3) Forma antigua (funciones sobre variables) vs tipos (preferible).
	var viejo int64
	atomic.AddInt64(&viejo, 3)
	fmt.Println("\nfunción atomic.AddInt64:            ", atomic.LoadInt64(&viejo))
	// Los tipos atómicos: (a) no se pueden usar sin atomic por accidente,
	// (b) garantizan alineación a 64 bits también en plataformas de 32 bits,
	// (c) go vet impide copiarlos.

	// 4) Restar con tipos sin signo: sumar el complemento a dos.
	var u atomic.Uint64
	u.Store(10)
	u.Add(^uint64(0)) // equivale a -1
	fmt.Println("Uint64: 10 - 1 =                    ", u.Load())

	// 5) atomic.Bool como bandera de parada.
	var parar atomic.Bool
	vueltas := atomic.Int64{}
	var wgW sync.WaitGroup
	wgW.Go(func() {
		for !parar.Load() {
			vueltas.Add(1)
			time.Sleep(time.Millisecond)
		}
	})
	time.Sleep(30 * time.Millisecond)
	parar.Store(true)
	wgW.Wait()
	fmt.Println("\nworker paró tras", vueltas.Load() > 0, "vueltas (bandera atómica)")

	// 6) Bandera "una sola vez" con CompareAndSwap: solo UNA goroutine gana.
	var iniciado atomic.Bool
	var ganadoras atomic.Int32
	for range 50 {
		wg.Go(func() {
			if iniciado.CompareAndSwap(false, true) {
				ganadoras.Add(1)
			}
		})
	}
	wg.Wait()
	fmt.Println("50 goroutines compitiendo, ganadoras:", ganadoras.Load())
}
