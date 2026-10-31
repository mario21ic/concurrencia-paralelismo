// Ejercicio 3: pánicos en goroutines.
//
//	go run .                 # todo recuperado
//	go run . -modo=crash     # un pánico en una goroutine MATA el proceso (exit 2)
//	go run . -modo=main      # recover en main NO atrapa pánicos de otras goroutines
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

var errBase = errors.New("índice fuera de rango")

func main() {
	modo := flag.String("modo", "seguro", "seguro|crash|main")
	flag.Parse()

	switch *modo {
	case "crash":
		go func() { panic("nadie me recupera") }()
		time.Sleep(time.Second)
		fmt.Println("no se llega aquí")
		return

	case "main":
		defer func() { fmt.Println("recover en main:", recover()) }() // NO atrapa el de la goroutine
		go func() { panic("pánico en otra goroutine") }()
		time.Sleep(time.Second)
		return
	}

	// 1) Capturar: pánico -> error con traza.
	err := Capturar(func() error {
		var m map[string]int
		m["x"] = 1 // escribir en un map nil
		return nil
	})
	pe, esPanic := EsPanic(err)
	fmt.Println("1) error:", err, "| es pánico:", esPanic)
	fmt.Println("   primera línea de la traza:", strings.SplitN(string(pe.Stack), "\n", 2)[0])

	// 2) panic(err) conserva la cadena: errors.Is funciona.
	err = Capturar(func() error { panic(fmt.Errorf("ctx: %w", errBase)) })
	fmt.Println("2) errors.Is(err, errBase):", errors.Is(err, errBase))

	// 3) Con errgroup: el pánico de una tarea cancela a las demás y llega como error.
	g, ctx := errgroup.WithContext(context.Background())
	for i := range 4 {
		GoSeguro(g, func() error {
			select {
			case <-time.After(time.Duration(i+1) * 30 * time.Millisecond):
				if i == 1 {
					panic(fmt.Sprintf("tarea %d explotó", i))
				}
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		})
	}
	err = g.Wait()
	_, esPanic = EsPanic(err)
	fmt.Println("3) Wait:", err, "| es pánico:", esPanic)

	// 4) Sin pánico no cambia nada.
	fmt.Println("4) función normal ->", Capturar(func() error { return nil }))

	// 5) runtime.Goexit y os.Exit no son pánicos: no los atrapa recover.
	fmt.Println("5) (ver README: Goexit ejecuta los defer pero recover() devuelve nil)")
}
