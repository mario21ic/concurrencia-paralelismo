// Ejercicio 4: time.Ticker.
package main

import (
	"fmt"
	"time"
)

func main() {
	// 1) Tick periódico y parada.
	fmt.Println("== 4 ticks cada 100ms ==")
	t := time.NewTicker(100 * time.Millisecond)
	ini := time.Now()
	for i := 1; i <= 4; i++ {
		<-t.C
		fmt.Printf("tick %d a los %v\n", i, time.Since(ini).Round(10*time.Millisecond))
	}
	t.Stop() // siempre Stop cuando ya no se usa

	// 2) Un consumidor lento NO acumula ticks: el ticker descarta los que no se leen.
	fmt.Println("\n== consumidor lento: ticker de 10ms, trabajo de 35ms, durante 300ms ==")
	t2 := time.NewTicker(10 * time.Millisecond)
	defer t2.Stop()
	fin := time.After(300 * time.Millisecond)
	recibidos := 0
	for {
		select {
		case <-t2.C:
			recibidos++
			time.Sleep(35 * time.Millisecond) // trabajo lento
		case <-fin:
			fmt.Printf("ticks recibidos: %d (teóricos: 30)\n", recibidos)
			goto heartbeat
		}
	}

heartbeat:
	// 3) Heartbeat con parada limpia mediante done.
	fmt.Println("\n== heartbeat con done ==")
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		tk := time.NewTicker(50 * time.Millisecond)
		defer tk.Stop()
		for {
			select {
			case <-tk.C:
				fmt.Println("latido")
			case <-done:
				fmt.Println("heartbeat detenido")
				return
			}
		}
	}()
	time.Sleep(175 * time.Millisecond)
	close(done)
	<-finished
}
