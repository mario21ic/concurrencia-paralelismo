// Ejercicio 2: la cena de los filósofos — cuatro estrategias frente al interbloqueo.
//
//	go run .
package main

import (
	"fmt"
	"time"
)

func main() {
	const n = 5
	const dur = 600 * time.Millisecond
	fmt.Printf("%d filósofos, %v de cena, comer = 1 ms\n\n", n, dur)

	for _, pausa := range []time.Duration{20 * time.Millisecond, 0} {
		fmt.Printf("== pausa entre el 1.º y el 2.º tenedor: %v ==\n", pausa)
		fmt.Printf("  %-18s %8s %-34s %8s %s\n", "estrategia", "comidas", "por filósofo", "equidad", "última comida")
		for _, e := range []Estrategia{Ingenua, Ordenada, Mayordomo, Asimetrica} {
			r := Cena(e, n, dur, time.Millisecond, pausa)
			estado := fmt.Sprintf("a los %v", r.UltimaComida.Round(time.Millisecond))
			if r.Total() == 0 {
				estado = "NUNCA (interbloqueo)"
			} else if r.UltimaComida < dur/2 {
				estado = fmt.Sprintf("a los %v → INTERBLOQUEADOS", r.UltimaComida.Round(time.Millisecond))
			}
			fmt.Printf("  %-18v %8d %-34s %8.2f %s\n", e, r.Total(), fmt.Sprint(r.Comidas), r.Equidad(), estado)
		}
		fmt.Println()
	}

	fmt.Println("== el bug latente: 'ingenua' sin pausa forzada; ¿cuántas de 100 cenas de 40 ms se interbloquean? ==")
	for _, comer := range []time.Duration{0, 100 * time.Microsecond, time.Millisecond} {
		bloqueadas := 0
		for range 100 {
			r := Cena(Ingenua, n, 40*time.Millisecond, comer, 0)
			if r.UltimaComida < 20*time.Millisecond {
				bloqueadas++
			}
		}
		fmt.Printf("  comer = %-6v → %3d de 100 cenas interbloqueadas\n", comer, bloqueadas)
	}
	fmt.Println("  El mismo código 'funciona' o falla según la temporización: por eso un test que pasa NO prueba que no haya deadlock.")
}
