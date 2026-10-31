// Ejercicio 5: Flight Recorder (runtime/trace, Go 1.25+).
//
// Un "flight recorder" guarda en memoria una ventana móvil de la traza de
// ejecución (los últimos segundos). Cuando ocurre algo anormal se vuelca esa
// ventana a un archivo: contiene lo que pasó ANTES y DURANTE el incidente, algo
// imposible con una traza normal (empezarla después del problema es tarde).
//
//	go run .
package main

import (
	"context"
	"fmt"
	"os"
	"runtime/trace"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mario21ic/golang-concurrencia-paralelismo/12-profiling/internal/traza"
)

const umbralLento = 150 * time.Millisecond

// base de datos simulada: un lock global.
var (
	bd         sync.Mutex
	mantenidas atomic.Int32
)

// mantenimiento mantiene el lock durante 300 ms cada 800 ms (p. ej. una compactación
// que hace E/S con el lock tomado). Es la CAUSA oculta de las peticiones lentas.
func mantenimiento(parar <-chan struct{}) {
	t := time.NewTicker(800 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			bd.Lock()
			mantenidas.Add(1)
			time.Sleep(300 * time.Millisecond)
			bd.Unlock()
		case <-parar:
			return
		}
	}
}

func main() {
	// Configurar el recorder: ventana de al menos 2 s y como máximo 4 MB.
	fr := trace.NewFlightRecorder(trace.FlightRecorderConfig{MinAge: 2 * time.Second, MaxBytes: 4 << 20})
	if err := fr.Start(); err != nil {
		panic(err)
	}
	defer fr.Stop()

	var volcadoHecho atomic.Bool
	var archivo atomic.Value // string

	// atender es el "handler": vigila su propia latencia y, ante el primer
	// incidente, vuelca la ventana de la traza.
	atender := func(ctx context.Context) {
		ctx, tarea := trace.NewTask(ctx, "pedido")
		defer tarea.End()
		ini := time.Now()
		trace.WithRegion(ctx, "consulta-bd", func() {
			bd.Lock()
			time.Sleep(200 * time.Microsecond) // consulta
			bd.Unlock()
		})
		d := time.Since(ini)

		if d > umbralLento && volcadoHecho.CompareAndSwap(false, true) { // solo el primer incidente
			trace.Logf(ctx, "alerta", "pedido lento: %v", d) // marca en la traza
			f, _ := os.CreateTemp("", "incidente-*.trace")
			n, err := fr.WriteTo(f)
			f.Close()
			if err == nil {
				archivo.Store(f.Name())
				fmt.Printf("¡incidente! pedido de %v: ventana volcada (%.0f KB)\n", d.Round(time.Millisecond), float64(n)/1024)
			}
		}
	}

	// Carga: 8 clientes durante 3 s, con el mantenimiento activo.
	parar := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() { mantenimiento(parar) })
	var total, lentas atomic.Int64
	fin := time.Now().Add(3 * time.Second)
	for range 8 {
		wg.Go(func() {
			for time.Now().Before(fin) {
				ini := time.Now()
				atender(context.Background())
				total.Add(1)
				if time.Since(ini) > umbralLento {
					lentas.Add(1)
				}
			}
		})
	}
	time.Sleep(3 * time.Second)
	close(parar)
	wg.Wait()
	fmt.Printf("peticiones: %d, lentas (> %v): %d, mantenimientos: %d\n\n", total.Load(), umbralLento, lentas.Load(), mantenidas.Load())

	nombre, _ := archivo.Load().(string)
	if nombre == "" {
		fmt.Println("no hubo incidente")
		return
	}
	defer os.Remove(nombre)

	f, _ := os.Open(nombre)
	defer f.Close()
	res, err := traza.Analizar(f)
	if err != nil {
		panic(err)
	}
	fmt.Println("== análisis de la ventana volcada ==")
	res.Imprimir(os.Stdout, 5)
	fmt.Println("\npara abrirla en el visor:  go tool trace", nombre, "(el archivo se borra al terminar este programa)")
}
