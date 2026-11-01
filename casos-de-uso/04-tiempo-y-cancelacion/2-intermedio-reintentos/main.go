// Caso 4.2 (intermedio): reintentar con backoff exponencial y jitter, sin reintentar lo que no se arregla.
//
// Problema: una llamada a una API falla a veces (503, timeout de red). Reintentar ayuda, pero:
//   - reintentar al instante y sin parar sobrecarga al servicio que ya sufre (y todos los clientes a la
//     vez crean una "estampida");
//   - reintentar un 400 (error del cliente) nunca va a funcionar;
//   - la espera entre intentos debe poder cancelarse (ctx), no dormir a ciegas con time.Sleep.
//
// Herramientas: time.Timer + select con ctx.Done, errors.Is para clasificar, jitter aleatorio.
//
//	go run .
package main

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"
)

// ErrTemporal marca los errores que merece la pena reintentar (red, 503, timeouts).
var ErrTemporal = errors.New("error temporal")

// Politica define cuántas veces y con qué esperas se reintenta.
type Politica struct {
	MaxIntentos int
	Base, Tope  time.Duration
	// Jitter transforma la espera calculada. Por defecto «equal jitter»: la mitad fija y la mitad
	// aleatoria, de modo que los clientes no coinciden. Los tests lo sustituyen por la identidad.
	Jitter func(time.Duration) time.Duration
}

func (p Politica) espera(intento int) time.Duration {
	d := p.Base
	for i := 1; i < intento && d < p.Tope; i++ {
		d *= 2
	}
	d = min(d, p.Tope)
	if p.Jitter != nil {
		return p.Jitter(d)
	}
	return d/2 + time.Duration(rand.Int64N(int64(d/2)+1))
}

// Reintentar ejecuta f hasta que tenga éxito, devuelva un error NO temporal, se agoten los intentos o
// se cancele ctx. Devuelve el último error envuelto con el nº de intentos.
func Reintentar(ctx context.Context, p Politica, f func(ctx context.Context) error) error {
	var err error
	for intento := 1; intento <= p.MaxIntentos; intento++ {
		if err = f(ctx); err == nil {
			return nil
		}
		if !errors.Is(err, ErrTemporal) {
			return fmt.Errorf("error permanente en el intento %d: %w", intento, err)
		}
		if intento == p.MaxIntentos {
			break
		}
		t := time.NewTimer(p.espera(intento))
		select {
		case <-t.C:
		case <-ctx.Done():
			t.Stop()
			return fmt.Errorf("cancelado tras %d intento(s): %w", intento, context.Cause(ctx))
		}
	}
	return fmt.Errorf("rendido tras %d intentos: %w", p.MaxIntentos, err)
}

func main() {
	llamadas := 0
	flaky := func(context.Context) error { // falla las 3 primeras veces
		llamadas++
		if llamadas <= 3 {
			return fmt.Errorf("503: %w", ErrTemporal)
		}
		return nil
	}
	p := Politica{MaxIntentos: 5, Base: 50 * time.Millisecond, Tope: time.Second}
	t0 := time.Now()
	err := Reintentar(context.Background(), p, flaky)
	fmt.Printf("servicio inestable: err=%v tras %d llamadas y %v (esperas ≈ 25-50, 50-100, 100-200 ms)\n", err, llamadas, time.Since(t0).Round(10*time.Millisecond))

	llamadas = 0
	err = Reintentar(context.Background(), p, func(context.Context) error { llamadas++; return errors.New("400: petición inválida") })
	fmt.Printf("error permanente:   %v (llamadas: %d, no se reintenta)\n", err, llamadas)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()
	err = Reintentar(ctx, p, func(context.Context) error { return fmt.Errorf("503: %w", ErrTemporal) })
	fmt.Printf("con plazo de 120 ms: %v\n", err)
}
