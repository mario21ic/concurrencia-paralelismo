// Ejercicio 6: runtime/metrics — observabilidad del planificador, el GC y los mutex.
//
//	go run .
package main

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

func cpu(d time.Duration) {
	fin := time.Now().Add(d)
	for time.Now().Before(fin) {
	}
}

type escenario struct {
	nombre string
	fn     func(parar <-chan struct{}, wg *sync.WaitGroup)
}

func main() {
	n := runtime.GOMAXPROCS(0)
	dur := time.Second

	escenarios := []escenario{
		{fmt.Sprintf("A) %d goroutines de CPU (= núcleos)", n), func(parar <-chan struct{}, wg *sync.WaitGroup) {
			for range n {
				wg.Go(func() {
					for {
						select {
						case <-parar:
							return
						default:
							cpu(time.Millisecond)
						}
					}
				})
			}
		}},
		{fmt.Sprintf("B) %d goroutines de CPU (%d× núcleos)", 32*n, 32), func(parar <-chan struct{}, wg *sync.WaitGroup) {
			for range 32 * n {
				wg.Go(func() {
					for {
						select {
						case <-parar:
							return
						default:
							cpu(time.Millisecond)
						}
					}
				})
			}
		}},
		{"C) 8 goroutines peleando por un mutex", func(parar <-chan struct{}, wg *sync.WaitGroup) {
			var mu sync.Mutex
			for range 8 {
				wg.Go(func() {
					for {
						select {
						case <-parar:
							return
						default:
							mu.Lock()
							cpu(200 * time.Microsecond) // sección crítica larga
							mu.Unlock()
						}
					}
				})
			}
		}},
		{"D) 2000 goroutines bloqueadas (sin CPU)", func(parar <-chan struct{}, wg *sync.WaitGroup) {
			for range 2000 {
				wg.Go(func() { <-parar })
			}
		}},
	}

	fmt.Printf("GOMAXPROCS=%d, cada escenario dura %v\n\n", n, dur)
	fmt.Printf("%-44s %9s %9s %9s %9s %9s %9s %8s\n",
		"escenario", "gor.máx", "listas", "espera.", "lat p50", "lat p99", "mutex(s)", "CPU %")

	for _, e := range escenarios {
		// Las métricas /cpu/classes/* se actualizan al FINAL de cada ciclo de GC (el
		// escenario no asigna memoria, así que no habría ciclos): se fuerza uno antes
		// de cada lectura para que reflejen el intervalo medido.
		runtime.GC()
		antes := Leer()
		parar := make(chan struct{})
		var wg sync.WaitGroup
		e.fn(parar, &wg)

		// Muestreador: máximo de goroutines totales y "listas" (esperando CPU) durante la ejecución.
		var maxGor, maxListas, maxEsp atomic.Uint64
		fin := make(chan struct{})
		go func() {
			t := time.NewTicker(20 * time.Millisecond)
			defer t.Stop()
			for {
				select {
				case <-t.C:
					f := Leer()
					maxGor.Store(max(maxGor.Load(), f.Goroutines))
					maxListas.Store(max(maxListas.Load(), f.Runnable))
					maxEsp.Store(max(maxEsp.Load(), f.Waiting))
				case <-fin:
					return
				}
			}
		}()

		time.Sleep(dur)
		fotoSched := Leer() // histograma y mutex ANTES del GC forzado (que pararía el mundo y falsearía la latencia)
		runtime.GC()
		despues := Leer() // solo para las métricas de CPU
		close(parar)
		wg.Wait()
		close(fin)

		lat := RestarHistograma(fotoSched.Latencias, antes.Latencias)
		cpuUsada := (despues.CPUTotal - despues.CPUIdle) - (antes.CPUTotal - antes.CPUIdle)
		cpuTotal := despues.CPUTotal - antes.CPUTotal

		fmt.Printf("%-44s %9d %9d %9d %9s %9s %9.2f %7.0f%%\n", e.nombre,
			maxGor.Load(), maxListas.Load(), maxEsp.Load(),
			dur2(Cuantil(lat, 0.50)), dur2(Cuantil(lat, 0.99)),
			fotoSched.MutexWait-antes.MutexWait, 100*cpuUsada/cpuTotal)
	}
	fmt.Println("\nlat p50/p99: tiempo que una goroutine lista espera para obtener un núcleo (cota superior del cubo del histograma).")
}

func dur2(seg float64) string {
	return time.Duration(seg * float64(time.Second)).Round(time.Microsecond).String()
}
