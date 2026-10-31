// Ejercicio 1: interbloqueo, livelock, inanición e inversión de prioridad.
//
//	go run .                          # recorre los escenarios recuperables
//	go run . -modo=deadlock-total     # el runtime lo detecta: "all goroutines are asleep" (fatal)
package main

import (
	"bytes"
	"flag"
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"runtime/pprof"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	modo := flag.String("modo", "todo", "todo|deadlock-total")
	flag.Parse()
	if *modo == "deadlock-total" {
		deadlockTotal()
		return
	}

	fmt.Println("== A) deadlock PARCIAL: el runtime NO lo detecta si hay otras goroutines vivas ==")
	deadlockParcial()

	fmt.Println("\n== B) dos soluciones al deadlock ==")
	soluciones()

	fmt.Println("\n== C) livelock: cortesía sin progreso (TryLock + soltar + reintentar al instante) ==")
	for _, n := range []int{2, 8} {
		f1, d1 := Reintentos(false, n)
		f2, d2 := Reintentos(true, n)
		fmt.Printf("  %d goroutines: sin backoff %6d fallos en %-8v | con backoff aleatorio %4d fallos en %v\n",
			n, f1, d1.Round(time.Millisecond), f2, d2.Round(time.Millisecond))
	}

	fmt.Println("\n== D) inversión de prioridad (1 'CPU', 10 tareas medias, 1 baja con el cerrojo, 1 alta que lo necesita) ==")
	sin := Inversion(false, 10)
	con := Inversion(true, 10)
	fmt.Printf("  sin herencia: la tarea ALTA tarda %v\n  con herencia: la tarea ALTA tarda %v   (%.1f× menos)\n",
		sin.Round(time.Millisecond), con.Round(time.Millisecond), float64(sin)/float64(con))

	fmt.Println("\n== E) inanición: una tarea baja de 1 turno frente a 4 flujos urgentes de 40 turnos ==")
	a := Inanicion(0, 4, 40)
	b := Inanicion(5*time.Millisecond, 4, 40)
	fmt.Printf("  sin envejecimiento: espera %v\n  con envejecimiento (+1 prio cada 5 ms): espera %v\n",
		a.Round(time.Millisecond), b.Round(time.Millisecond))
}

func deadlockTotal() {
	a, b := &Cuenta{ID: 1, Saldo: 100}, &Cuenta{ID: 2, Saldo: 100}
	var wg sync.WaitGroup
	wg.Go(func() { TransferirMal(a, b, 10, 10*time.Millisecond) })
	wg.Go(func() { TransferirMal(b, a, 10, 10*time.Millisecond) })
	fmt.Println("dos transferencias cruzadas; main espera con wg.Wait()...")
	wg.Wait()
}

// deadlockParcial provoca el interbloqueo con una goroutine "viva" en paralelo (un ticker, como
// en cualquier servidor real) y lo detecta con un vigilante de progreso + volcado de goroutines.
func deadlockParcial() {
	a, b := &Cuenta{ID: 1, Saldo: 100}, &Cuenta{ID: 2, Saldo: 100}
	var progreso atomic.Int64
	parar := make(chan struct{})
	go func() { // goroutine ajena que mantiene vivo el proceso
		t := time.NewTicker(50 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-t.C:
			case <-parar:
				return
			}
		}
	}()
	go func() { TransferirMal(a, b, 10, 20*time.Millisecond); progreso.Add(1) }()
	go func() { TransferirMal(b, a, 10, 20*time.Millisecond); progreso.Add(1) }()

	time.Sleep(300 * time.Millisecond)
	fmt.Printf("  tras 300 ms: transferencias completadas = %d de 2 (el runtime sigue sin quejarse)\n", progreso.Load())
	var buf bytes.Buffer
	pprof.Lookup("goroutine").WriteTo(&buf, 1)
	fmt.Println("  perfil de goroutines (debug=1), solo las bloqueadas en un Mutex:")
	for bloque := range strings.SplitSeq(buf.String(), "\n\n") {
		if strings.Contains(bloque, "sync.(*Mutex).Lock") || strings.Contains(bloque, "sync.runtime_SemacquireMutex") {
			lineas := strings.Split(strings.TrimSpace(bloque), "\n")
			fmt.Printf("    %s goroutine(s):\n", strings.Fields(lineas[0])[0])
			for _, l := range lineas[1:] {
				if c := strings.Split(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(l), "#")), "\t"); len(c) >= 3 && strings.Contains(l, "main.Transferir") {
					fmt.Printf("      bloqueada en %s (%s)\n", strings.Split(c[1], "+")[0], filepath.Base(strings.TrimSpace(c[len(c)-1])))
					break
				}
			}
		}
	}
	close(parar)
}

func soluciones() {
	for _, c := range []struct {
		nombre string
		f      func(a, b *Cuenta) bool
	}{
		{"orden global de cerrojos", func(a, b *Cuenta) bool { TransferirOrdenado(a, b, 10, 10*time.Millisecond); return true }},
		{"TryLock con plazo + reintento con jitter", func(a, b *Cuenta) bool {
			for !TransferirConPlazo(a, b, 10, 10*time.Millisecond, 3*time.Millisecond) {
				time.Sleep(time.Duration(rand.IntN(20)) * time.Millisecond)
			}
			return true
		}},
	} {
		a, b := &Cuenta{ID: 1, Saldo: 100}, &Cuenta{ID: 2, Saldo: 100}
		var okA, okB bool
		var wg sync.WaitGroup
		wg.Go(func() { okA = c.f(a, b) })
		wg.Go(func() { okB = c.f(b, a) })
		wg.Wait()
		fmt.Printf("  %-42s transferencias: %v y %v | saldos %d + %d = %d\n", c.nombre, okA, okB, a.Saldo, b.Saldo, a.Saldo+b.Saldo)
	}
	fmt.Println("  (sin el jitter, ambas abortarían a la vez una y otra vez: solo cambiaría el deadlock por un livelock)")
}
