// Caso 1.3 (avanzado): consultar varios servicios a la vez, con plazo global, y degradar con elegancia.
//
// Problema: el panel de un usuario necesita 3 servicios. Dos son ESENCIALES (usuario, pedidos); el
// tercero (recomendaciones) es opcional. Si el opcional tarda o falla, el panel debe salir igualmente,
// avisando de que está incompleto. Si falla un esencial, no tiene sentido seguir.
//
// Herramientas: errgroup (esperar y propagar), context.WithTimeout (plazo de todo el panel) y la
// decisión de DISEÑO de qué errores abortan y cuáles se anotan. errgroup por sí solo no distingue.
//
//	go run .
package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
)

type Panel struct {
	Usuario         string
	Pedidos         []string
	Recomendaciones []string
	Incompleto      []string // qué partes opcionales faltan y por qué
}

type Servicios struct {
	Usuario         func(ctx context.Context) (string, error)
	Pedidos         func(ctx context.Context) ([]string, error)
	Recomendaciones func(ctx context.Context) ([]string, error)
}

// Armar consulta los tres a la vez.
//   - Esenciales: su error cancela al resto y se devuelve (errgroup).
//   - Opcional: su error NO cancela a nadie; se anota en Incompleto. Se le da un plazo propio más corto
//     que el global para que un servicio lento no consuma todo el presupuesto de tiempo.
func Armar(ctx context.Context, s Servicios, plazo, plazoOpcional time.Duration) (Panel, error) {
	ctx, cancel := context.WithTimeout(ctx, plazo)
	defer cancel()

	var p Panel
	var mu sync.Mutex // protege p.Incompleto (las otras dos escrituras son a campos distintos)
	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() (err error) {
		p.Usuario, err = s.Usuario(gctx)
		return
	})
	g.Go(func() (err error) {
		p.Pedidos, err = s.Pedidos(gctx)
		return
	})
	g.Go(func() error { // opcional: siempre devuelve nil
		octx, cancelOpc := context.WithTimeout(gctx, plazoOpcional)
		defer cancelOpc()
		r, err := s.Recomendaciones(octx)
		if err != nil {
			mu.Lock()
			p.Incompleto = append(p.Incompleto, "recomendaciones: "+err.Error())
			mu.Unlock()
			return nil
		}
		p.Recomendaciones = r
		return nil
	})
	if err := g.Wait(); err != nil {
		return Panel{}, err
	}
	return p, nil
}

func tardar[T any](d time.Duration, v T, err error) func(context.Context) (T, error) {
	return func(ctx context.Context) (T, error) {
		select {
		case <-time.After(d):
			return v, err
		case <-ctx.Done():
			var cero T
			return cero, ctx.Err()
		}
	}
}

func main() {
	base := Servicios{
		Usuario: tardar(50*time.Millisecond, "ana", nil),
		Pedidos: tardar(80*time.Millisecond, []string{"#1", "#2"}, nil),
	}
	casos := []struct {
		nombre  string
		recom   func(context.Context) ([]string, error)
		pedidos func(context.Context) ([]string, error)
	}{
		{"todo bien", tardar(60*time.Millisecond, []string{"libro"}, nil), base.Pedidos},
		{"recomendaciones lentas (opcional)", tardar(5*time.Second, []string{"libro"}, nil), base.Pedidos},
		{"recomendaciones fallan (opcional)", tardar[[]string](10*time.Millisecond, nil, errors.New("503")), base.Pedidos},
		{"pedidos fallan (esencial)", tardar(60*time.Millisecond, []string{"libro"}, nil), tardar[[]string](20*time.Millisecond, nil, errors.New("BD caída"))},
	}
	for _, c := range casos {
		s := base
		s.Recomendaciones, s.Pedidos = c.recom, c.pedidos
		t0 := time.Now()
		p, err := Armar(context.Background(), s, time.Second, 200*time.Millisecond)
		fmt.Printf("%-36s %6v  err=%v  panel=%+v\n", c.nombre, time.Since(t0).Round(10*time.Millisecond), err, p)
	}
}
