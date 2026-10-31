// Mini-proyecto 2: caché con TTL.
//
//	go run .
package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	c := Nueva[string, int](ctx, 20*time.Millisecond)

	c.Set("corta", 1, 50*time.Millisecond)
	c.Set("larga", 2, time.Second)
	c.Set("eterna", 3, 0)

	v, ok := c.Get("corta")
	fmt.Println("Get corta (recién creada):", v, ok)

	time.Sleep(120 * time.Millisecond)
	fmt.Println("Len tras 120ms (sin llamar a Get):", c.Len(), "(el barredor ya quitó 'corta')")
	_, ok = c.Get("corta")
	fmt.Println("Get corta tras 120ms:             ", ok)

	// Carga concurrente: 8 goroutines leen y escriben.
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Go(func() {
			for i := range 10_000 {
				if i%4 == 0 {
					c.Set(fmt.Sprintf("k%d", (i/4)%100), g, time.Second)
				} else {
					c.Get(fmt.Sprintf("k%d", i%100))
				}
			}
		})
	}
	wg.Wait()
	s := c.Stats()
	fmt.Printf("stats: aciertos=%d fallos=%d expiradas=%d (len=%d)\n", s.Aciertos, s.Fallos, s.Expiradas, c.Len())

	// Cancelar el contexto detiene al barredor.
	cancel()
	select {
	case <-c.Hecho():
		fmt.Println("barredor detenido tras cancelar el contexto")
	case <-time.After(time.Second):
		fmt.Println("¡el barredor no terminó!")
	}
}
