// Ejercicio 2: bucles CompareAndSwap (actualización optimista).
//
// Patrón: leer -> calcular -> CAS(viejo, nuevo); si falla, otro cambió el
// valor entre medias: reintentar con el valor nuevo.
package main

import (
	"fmt"
	"math/rand/v2"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
)

// registrarMax mantiene el máximo observado sin locks.
func registrarMax(max *atomic.Int64, v int64) {
	for {
		actual := max.Load()
		if v <= actual {
			return // ya hay uno mayor o igual: nada que hacer
		}
		if max.CompareAndSwap(actual, v) {
			return
		}
		// otro cambió `max` entre Load y CAS: reintentar
	}
}

// Saldo con retirada "si hay fondos" lock-free. Es la versión atómica del
// check-then-act del módulo 6, ejercicio 7: comprobar y restar en UN solo CAS.
type Saldo struct{ v atomic.Int64 }

func (s *Saldo) Retirar(n int64) bool {
	for {
		actual := s.v.Load()
		if actual < n {
			return false
		}
		if s.v.CompareAndSwap(actual, actual-n) {
			return true
		}
	}
}

// Spinlock con CAS: SOLO para entender; en la práctica usa sync.Mutex.
type Spinlock struct{ tomado atomic.Bool }

func (l *Spinlock) Lock() {
	for !l.tomado.CompareAndSwap(false, true) {
		runtime.Gosched() // sin esto, con pocos núcleos gasta CPU sin ceder
	}
}
func (l *Spinlock) Unlock() { l.tomado.Store(false) }

func main() {
	var wg sync.WaitGroup

	// 1) Máximo concurrente.
	var max atomic.Int64
	var mu sync.Mutex
	var valores []int64
	for g := range 100 {
		wg.Go(func() {
			r := rand.New(rand.NewPCG(uint64(g), 7))
			for range 1000 {
				v := r.Int64N(1_000_000)
				registrarMax(&max, v)
				mu.Lock()
				valores = append(valores, v)
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	real := slices.Max(valores) // verificación secuencial
	fmt.Printf("máximo con CAS: %d | máximo real: %d | coincide: %v\n", max.Load(), real, max.Load() == real)

	// 2) Retirada atómica "si hay fondos": nunca queda negativo ni se retira de más.
	var s Saldo
	s.v.Store(1000)
	var exitos atomic.Int64
	for range 500 {
		wg.Go(func() {
			if s.Retirar(7) {
				exitos.Add(1)
			}
		})
	}
	wg.Wait()
	fmt.Printf("saldo 1000, 500 intentos de retirar 7: éxitos=%d (máx. posible %d), saldo final=%d\n",
		exitos.Load(), 1000/7, s.v.Load())

	// 3) Spinlock.
	var sl Spinlock
	contador := 0
	for range 100 {
		wg.Go(func() {
			for range 1000 {
				sl.Lock()
				contador++
				sl.Unlock()
			}
		})
	}
	wg.Wait()
	fmt.Println("spinlock: contador =", contador, "(esperado 100000)")
}
