// Caso 10.2 (intermedio): paralelismo POR FASES — una simulación que avanza por generaciones.
//
// Problema: el Juego de la Vida de Conway. La generación t+1 depende de TODA la generación t, así
// que no se puede repartir "el tiempo": hay que avanzar de generación en generación. Lo que sí se
// puede repartir es el espacio dentro de cada generación:
//
//	generación t ──► [ banda 0 | banda 1 | … | banda k−1 ]  (cada worker, sus filas)
//	                              │ barrera (wg.Wait) │
//	generación t+1 ◄── intercambiar tableros (doble búfer)
//
// Dos reglas: (1) DOBLE BÚFER: se lee de `act` y se escribe en `sig`, nunca en el mismo tablero,
// o los vecinos leerían celdas ya actualizadas; (2) BARRERA entre generaciones: nadie empieza la
// t+1 hasta que todas las bandas de la t han terminado (aquí, el propio `wg.Wait()`).
//
// Granularidad: coordinar cuesta microsegundos por generación. Con tableros pequeños no compensa.
//
//	go run .
package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

// Tablero es una malla toroidal (los bordes se unen) con una celda por byte: 0 = muerta, 1 = viva.
type Tablero struct {
	n     int
	celda []byte
}

func Nuevo(n int) *Tablero { return &Tablero{n: n, celda: make([]byte, n*n)} }

// Aleatorio rellena con un generador congruencial fijo (determinista, ≈ 30 % de celdas vivas).
func (t *Tablero) Aleatorio(semilla uint32) {
	x := semilla
	for i := range t.celda {
		x = x*1664525 + 1013904223
		if x>>24 < 77 {
			t.celda[i] = 1
		} else {
			t.celda[i] = 0
		}
	}
}

func (t *Tablero) Vivas() (n int) {
	for _, c := range t.celda {
		n += int(c)
	}
	return
}

// banda calcula las filas [desde, hasta) de la siguiente generación leyendo de t y escribiendo en sig.
func (t *Tablero) banda(sig *Tablero, desde, hasta int) {
	n := t.n
	for y := desde; y < hasta; y++ {
		arriba := t.celda[((y+n-1)%n)*n : ((y+n-1)%n+1)*n]
		medio := t.celda[y*n : (y+1)*n]
		abajo := t.celda[((y+1)%n)*n : ((y+1)%n+1)*n]
		dest := sig.celda[y*n : (y+1)*n]
		for x := range n {
			izq, der := (x+n-1)%n, (x+1)%n
			vecinas := arriba[izq] + arriba[x] + arriba[der] + medio[izq] + medio[der] + abajo[izq] + abajo[x] + abajo[der]
			dest[x] = 0
			if vecinas == 3 || (vecinas == 2 && medio[x] == 1) {
				dest[x] = 1
			}
		}
	}
}

// Evoluciona avanza `gens` generaciones con k workers (k = 1 → secuencial, sin goroutines) y devuelve
// el tablero resultante. Usa el doble búfer: intercambia los dos tableros tras cada generación.
func Evoluciona(inicial *Tablero, gens, k int) *Tablero {
	k = max(1, min(k, inicial.n))
	act, sig := Nuevo(inicial.n), Nuevo(inicial.n)
	copy(act.celda, inicial.celda)
	for range gens {
		if k == 1 {
			act.banda(sig, 0, act.n)
		} else {
			var wg sync.WaitGroup
			for w := range k {
				wg.Go(func() { act.banda(sig, w*act.n/k, (w+1)*act.n/k) })
			}
			wg.Wait() // BARRERA: la generación t+1 no empieza hasta que terminen todas las bandas de la t
		}
		act, sig = sig, act
	}
	return act
}

func mejor(veces int, f func()) time.Duration {
	m := time.Duration(1 << 62)
	for range veces {
		t0 := time.Now()
		f()
		m = min(m, time.Since(t0))
	}
	return m
}

func main() {
	fmt.Printf("GOMAXPROCS=%d. Juego de la Vida toroidal, mejor de 3. Aceleración frente a k=1.\n\n", runtime.GOMAXPROCS(0))
	fmt.Printf("%-18s %-12s %-12s %-16s %-16s %-16s\n", "tablero", "generaciones", "k=1", "k=2", "k=4", "k=8")
	for _, c := range []struct{ n, gens int }{{32, 20000}, {128, 5000}, {512, 500}, {2048, 40}} {
		t := Nuevo(c.n)
		t.Aleatorio(1)
		base := mejor(3, func() { Evoluciona(t, c.gens, 1) })
		fmt.Printf("%-18s %-12d %-12v", fmt.Sprintf("%d×%d", c.n, c.n), c.gens, base.Round(100*time.Microsecond))
		for _, k := range []int{2, 4, 8} {
			d := mejor(3, func() { Evoluciona(t, c.gens, k) })
			fmt.Printf(" %-16s", fmt.Sprintf("%v (%.1f×)", d.Round(100*time.Microsecond), float64(base)/float64(d)))
		}
		fmt.Println()
	}
}
