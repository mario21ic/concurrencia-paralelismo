// Ejercicio 4: traza de ejecución (runtime/trace) con tareas y regiones.
//
//	go run . -workers 8 -salida trace.out
//	go tool trace trace.out                      # visor en el navegador
//	go tool trace -pprof=sync  trace.out > sync.pprof   && go tool pprof -top sync.pprof
//	go tool trace -pprof=sched trace.out > sched.pprof  && go tool pprof -top sched.pprof
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime/trace"
	"sync"
	"time"

	"github.com/mario21ic/golang-concurrencia-paralelismo/12-profiling/internal/traza"
)

var guardado sync.Mutex // recurso compartido: la sección "guardar" se serializa

func busy(d time.Duration) {
	fin := time.Now().Add(d)
	for time.Now().Before(fin) {
	}
}

// procesar atraviesa tres fases; cada una es una REGIÓN dentro de la TAREA "pedido".
func procesar(ctx context.Context, id int) {
	ctx, tarea := trace.NewTask(ctx, "pedido") // una tarea agrupa el trabajo de una petición
	defer tarea.End()
	trace.Logf(ctx, "pedido", "id=%d", id)

	trace.WithRegion(ctx, "validar", func() { busy(300 * time.Microsecond) }) // CPU

	trace.WithRegion(ctx, "esperar-bd", func() { time.Sleep(2 * time.Millisecond) }) // E/S simulada

	trace.WithRegion(ctx, "guardar", func() { // sección crítica compartida
		guardado.Lock()
		defer guardado.Unlock()
		busy(400 * time.Microsecond)
	})
}

func ejecutar(workers, pedidos int) time.Duration {
	ctx := context.Background()
	trabajos := make(chan int)
	var wg sync.WaitGroup
	ini := time.Now()
	for range workers {
		wg.Go(func() {
			for id := range trabajos {
				procesar(ctx, id)
			}
		})
	}
	for i := range pedidos {
		trabajos <- i
	}
	close(trabajos)
	wg.Wait()
	return time.Since(ini)
}

func main() {
	workers := flag.Int("workers", 8, "workers")
	pedidos := flag.Int("pedidos", 400, "pedidos")
	salida := flag.String("salida", "trace.out", "archivo de traza")
	flag.Parse()

	// Sin traza (referencia para medir el coste de trazar).
	base := ejecutar(*workers, *pedidos)

	f, err := os.Create(*salida)
	if err != nil {
		panic(err)
	}
	if err := trace.Start(f); err != nil {
		panic(err)
	}
	conTraza := ejecutar(*workers, *pedidos)
	trace.Stop()
	f.Close()

	st, _ := os.Stat(*salida)
	fmt.Printf("sin traza: %v | con traza: %v | archivo %s: %.0f KB\n\n",
		base.Round(time.Millisecond), conTraza.Round(time.Millisecond), *salida, float64(st.Size())/1024)

	f, _ = os.Open(*salida)
	defer f.Close()
	res, err := traza.Analizar(f)
	if err != nil {
		panic(err)
	}
	res.Imprimir(os.Stdout, 6)
}
