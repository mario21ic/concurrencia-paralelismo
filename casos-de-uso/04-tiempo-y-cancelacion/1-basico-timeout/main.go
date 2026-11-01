// Caso 4.1 (básico): no esperar para siempre — poner un plazo a una operación lenta.
//
// Problema: una llamada a un servicio puede tardar segundos (o colgarse). El usuario no espera más de
// 200 ms. Tres formas de hacerlo, de peor a mejor:
//
//  1. ConPlazoMal:   goroutine + select + time.After con channel SIN buffer → FUGA de goroutine.
//
//  2. ConPlazoSinCtx: igual, pero con buffer 1: no fuga, aunque la operación sigue gastando recursos.
//
//  3. ConPlazoCtx:    la operación RECIBE el contexto y se cancela de verdad. Es la forma correcta.
//
//     go run .
package main

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"time"
)

var ErrPlazo = errors.New("plazo vencido")

// 1. Mal: si vence el plazo, nadie lee `res` y la goroutine se queda bloqueada enviando PARA SIEMPRE.
func ConPlazoMal(d time.Duration, f func() string) (string, error) {
	res := make(chan string) // ← sin buffer
	go func() { res <- f() }()
	select {
	case v := <-res:
		return v, nil
	case <-time.After(d):
		return "", ErrPlazo
	}
}

// 2. Aceptable cuando f NO se puede cancelar (código ajeno, cgo): con buffer 1 la goroutine envía y
// termina aunque nadie la espere. La operación sigue ejecutándose hasta el final (no se puede evitar).
func ConPlazoSinCtx(d time.Duration, f func() string) (string, error) {
	res := make(chan string, 1) // ← buffer 1: el envío nunca bloquea
	go func() { res <- f() }()
	t := time.NewTimer(d)
	defer t.Stop() // libera el temporizador si f termina antes
	select {
	case v := <-res:
		return v, nil
	case <-t.C:
		return "", ErrPlazo
	}
}

// 3. Correcto: el contexto viaja hasta la operación; al vencer el plazo, la operación se entera y
// libera sus recursos (conexión, CPU). Sin goroutines extra.
func ConPlazoCtx(ctx context.Context, d time.Duration, f func(ctx context.Context) (string, error)) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, d)
	defer cancel()
	v, err := f(ctx)
	if errors.Is(err, context.DeadlineExceeded) {
		return "", ErrPlazo
	}
	return v, err
}

func lenta() string { time.Sleep(time.Second); return "respuesta" }

func lentaCtx(ctx context.Context) (string, error) {
	select {
	case <-time.After(time.Second):
		return "respuesta", nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func main() {
	antes := runtime.NumGoroutine()
	for range 100 {
		ConPlazoMal(10*time.Millisecond, lenta)
	}
	time.Sleep(1200 * time.Millisecond) // más que lo que tarda `lenta`: ya deberían haber terminado
	fmt.Printf("ConPlazoMal   ×100: goroutines que SOBREVIVEN = %d (bloqueadas enviando)\n", runtime.NumGoroutine()-antes)

	antes = runtime.NumGoroutine()
	for range 100 {
		ConPlazoSinCtx(10*time.Millisecond, lenta)
	}
	time.Sleep(1200 * time.Millisecond)
	fmt.Printf("ConPlazoSinCtx×100: goroutines que sobreviven = %d\n", runtime.NumGoroutine()-antes)

	t0 := time.Now()
	_, err := ConPlazoCtx(context.Background(), 200*time.Millisecond, lentaCtx)
	fmt.Printf("ConPlazoCtx: error=%v tras %v (la operación se canceló, no quedó trabajando)\n", err, time.Since(t0).Round(10*time.Millisecond))
}
