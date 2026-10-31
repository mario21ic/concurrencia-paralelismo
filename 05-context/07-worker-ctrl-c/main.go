// Ejercicio 7: worker que se detiene limpiamente con Ctrl+C.
//
//	go run . -workers 3
//	go run . -workers 3 -abortar        # cancela también el trabajo en curso
//	go run . -duracion 1s               # se apaga solo (útil para probar)
//
// Primera señal (SIGINT/SIGTERM): apagado ordenado.
// Segunda señal: terminación inmediata (stop() restaura el comportamiento por defecto).
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var completados, abortados atomic.Int64

// procesar simula un trabajo de 5 pasos de 60 ms.
// Con abortar=true revisa el contexto entre pasos.
func procesar(ctx context.Context, id int, abortar bool) {
	for paso := 1; paso <= 5; paso++ {
		if abortar && ctx.Err() != nil {
			abortados.Add(1)
			fmt.Printf("  trabajo %d abortado en el paso %d\n", id, paso)
			return
		}
		time.Sleep(60 * time.Millisecond)
	}
	completados.Add(1)
}

func productor(ctx context.Context, jobs chan<- int) {
	defer close(jobs) // el productor es dueño del cierre
	t := time.NewTicker(100 * time.Millisecond)
	defer t.Stop()
	for id := 1; ; id++ {
		select {
		case <-t.C:
		case <-ctx.Done():
			fmt.Println("productor: no se aceptan más trabajos")
			return
		}
		select {
		case jobs <- id:
		case <-ctx.Done():
			fmt.Println("productor: no se aceptan más trabajos")
			return
		}
	}
}

func main() {
	workers := flag.Int("workers", 3, "número de workers")
	abortar := flag.Bool("abortar", false, "abortar el trabajo en curso al cancelar")
	duracion := flag.Duration("duracion", 0, "apagarse sola tras este tiempo (0 = esperar señal)")
	gracia := flag.Duration("gracia", 2*time.Second, "tiempo máximo para el apagado ordenado")
	flag.Parse()

	// El contexto se cancela con SIGINT (Ctrl+C) o SIGTERM (kill, Kubernetes).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *duracion > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *duracion)
		defer cancel()
	}

	fmt.Printf("pid=%d, %d workers. Pulsa Ctrl+C para apagar.\n", os.Getpid(), *workers)

	jobs := make(chan int)
	go productor(ctx, jobs)

	var wg sync.WaitGroup
	for w := 1; w <= *workers; w++ {
		wg.Go(func() {
			for id := range jobs { // termina cuando el productor cierra jobs
				fmt.Printf("worker %d procesando trabajo %d\n", w, id)
				procesar(ctx, id, *abortar)
			}
			fmt.Printf("worker %d terminó\n", w)
		})
	}

	<-ctx.Done()
	fmt.Printf("\napagando (%v)... un segundo Ctrl+C fuerza la salida\n", context.Cause(ctx))
	stop() // restaura el manejo por defecto: la 2.ª señal mata el proceso

	hecho := make(chan struct{})
	go func() { wg.Wait(); close(hecho) }()
	select {
	case <-hecho:
		fmt.Println("apagado ordenado completado")
	case <-time.After(*gracia):
		fmt.Println("tiempo de gracia agotado: salida forzada")
		os.Exit(1)
	}
	fmt.Printf("resumen: %d completados, %d abortados\n", completados.Load(), abortados.Load())
}
