// Caso 1.2 (intermedio): lanzar un lote, parar todo al primer error y no pasar de N a la vez.
//
// Problema: validar 20 ficheros contra un servicio externo. Si UNO es inválido, seguir con los demás
// es trabajo perdido; y lanzar los 20 a la vez puede saturar el servicio.
// Herramienta: golang.org/x/sync/errgroup (WithContext + SetLimit).
//
//	go run .
package main

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

type Validador func(ctx context.Context, item string) error

// ValidarTodos valida todos los items con a lo sumo `limite` en vuelo.
//   - El primer error cancela el contexto: las validaciones en curso lo ven (ctx.Done) y se abandonan.
//   - Wait() espera a TODAS las goroutines (ninguna sobrevive a la función) y devuelve ESE primer error.
func ValidarTodos(ctx context.Context, items []string, limite int, validar Validador) error {
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(limite) // g.Go BLOQUEA cuando ya hay `limite` en curso: contrapresión al lanzar
	for _, it := range items {
		g.Go(func() error {
			if err := validar(ctx, it); err != nil {
				return fmt.Errorf("%s: %w", it, err)
			}
			return nil
		})
	}
	return g.Wait()
}

var errInvalido = errors.New("formato inválido")

func main() {
	var activas, maxActivas, canceladas atomic.Int32
	validar := func(ctx context.Context, it string) error {
		n := activas.Add(1)
		defer activas.Add(-1)
		for { // máximo observado de validaciones simultáneas
			m := maxActivas.Load()
			if n <= m || maxActivas.CompareAndSwap(m, n) {
				break
			}
		}
		if it == "f02" {
			time.Sleep(50 * time.Millisecond)
			return errInvalido
		}
		select {
		case <-time.After(500 * time.Millisecond): // validación lenta
			return nil
		case <-ctx.Done(): // otra falló: nos cancelan
			canceladas.Add(1)
			return ctx.Err()
		}
	}

	items := make([]string, 20)
	for i := range items {
		items[i] = fmt.Sprintf("f%02d", i+1)
	}
	t0 := time.Now()
	err := ValidarTodos(context.Background(), items, 4, validar)
	fmt.Printf("error: %v\n", err)
	fmt.Printf("tardó %v (sin cancelar habría sido ≈ %v) | máx. simultáneas: %d | vieron la cancelación: %d (en curso o por empezar)\n",
		time.Since(t0).Round(10*time.Millisecond), 5*500*time.Millisecond, maxActivas.Load(), canceladas.Load())
}
