// Ejercicio 6: limitación de tasa (rate limiting).
package main

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/time/rate"
)

func marca(ini time.Time) string { return time.Since(ini).Round(10 * time.Millisecond).String() }

func main() {
	ctx := context.Background()

	// 1) Ticker: la forma más simple. Sin ráfagas: una operación por tick.
	fmt.Println("== 1) time.Ticker, 1 operación cada 100ms ==")
	t := time.NewTicker(100 * time.Millisecond)
	ini := time.Now()
	for i := 1; i <= 5; i++ {
		<-t.C
		fmt.Printf("  op %d a los %s\n", i, marca(ini))
	}
	t.Stop()

	// 2) Token bucket propio: 10 ops/s con ráfaga de 3.
	fmt.Println("\n== 2) token bucket propio: 10/s, burst 3 ==")
	b := NuevoTokenBucket(10, 3)
	ini = time.Now()
	for i := 1; i <= 8; i++ {
		b.Wait(ctx)
		fmt.Printf("  op %d a los %s\n", i, marca(ini))
	}

	// 3) golang.org/x/time/rate: lo mismo, listo para producción.
	fmt.Println("\n== 3) x/time/rate: 10/s, burst 3 ==")
	lim := rate.NewLimiter(rate.Limit(10), 3)
	ini = time.Now()
	for i := 1; i <= 8; i++ {
		lim.Wait(ctx)
		fmt.Printf("  op %d a los %s\n", i, marca(ini))
	}

	// 4) Allow: rechazar en lugar de esperar (load shedding / HTTP 429).
	fmt.Println("\n== 4) Allow (no bloquea): 2/s, burst 2 ==")
	lim2 := rate.NewLimiter(rate.Limit(2), 2)
	for i := 1; i <= 4; i++ {
		fmt.Printf("  petición %d permitida: %v\n", i, lim2.Allow())
	}
	time.Sleep(520 * time.Millisecond) // ~1 token repuesto
	fmt.Printf("  tras 520ms, permitida: %v\n", lim2.Allow())

	// 5) Reserve: saber CUÁNTO habría que esperar sin bloquear.
	lim3 := rate.NewLimiter(rate.Every(200*time.Millisecond), 1)
	lim3.Allow() // consume el único token
	r := lim3.Reserve()
	fmt.Printf("\n== 5) Reserve: habría que esperar ~%v ==\n", r.Delay().Round(10*time.Millisecond))
	r.Cancel() // no vamos a usarlo: devolver el token

	// 6) Wait con plazo: no esperar más de lo razonable.
	lim4 := rate.NewLimiter(rate.Every(time.Second), 1)
	lim4.Allow()
	ctxPlazo, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
	defer cancel()
	fmt.Println("\n== 6) Wait con plazo de 100ms (el siguiente token tarda 1s) ==")
	fmt.Println(" ", lim4.Wait(ctxPlazo))
}
