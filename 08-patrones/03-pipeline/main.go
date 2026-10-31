// Ejercicio 3: pipeline con cancelación.
//
//	Contar ──► Filtrar(pares) ──► Mapear(cuadrado) ──► Tomar(5) ──► consumidor
package main

import (
	"context"
	"fmt"
	"runtime"
	"time"
)

func pares(n int) bool     { return n%2 == 0 }
func cuadrado(n int) int   { return n * n }
func esperar()             { time.Sleep(50 * time.Millisecond) }
func fugadas(base int) int { return runtime.NumGoroutine() - base }

func main() {
	base := runtime.NumGoroutine()

	// 1) Con cancelación: al tener lo que necesita, el consumidor cancela y todo termina.
	ctx, cancel := context.WithCancel(context.Background())
	p := Tomar(ctx, Mapear(ctx, Filtrar(ctx, Contar(ctx), pares), cuadrado), 5)
	fmt.Print("primeros 5 cuadrados de pares: ")
	for v := range p {
		fmt.Print(v, " ")
	}
	fmt.Println()
	fmt.Println("goroutines activas antes de cancelar:", fugadas(base), "(la fuente sigue produciendo)")
	cancel()
	esperar()
	fmt.Println("goroutines activas tras cancelar:    ", fugadas(base))

	// 2) Sin cancelar: Tomar termina pero las etapas anteriores quedan bloqueadas.
	bg := context.Background()
	p2 := Tomar(bg, Mapear(bg, Filtrar(bg, Contar(bg), pares), cuadrado), 5)
	for range p2 {
	}
	esperar()
	fmt.Println("\nsin cancelar, goroutines fugadas:   ", fugadas(base), "(3 etapas bloqueadas para siempre)")

	// 3) Errores como datos: el pipeline no se aborta por una entrada mala.
	ctx3, cancel3 := context.WithCancel(context.Background())
	defer cancel3()
	fmt.Println("\nparseo con errores:")
	for r := range Parsear(ctx3, Desde(ctx3, "1", "2", "x", "4")) {
		if r.Err != nil {
			fmt.Println("  error:", r.Err)
			continue
		}
		fmt.Println("  valor:", r.Val)
	}
}
