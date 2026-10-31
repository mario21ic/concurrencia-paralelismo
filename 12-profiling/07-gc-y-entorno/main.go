// Ejercicio 7: GC, GOGC, GOMEMLIMIT y su efecto en un servicio concurrente.
//
//	go run . -gogc 100
//	go run . -gogc 400
//	go run . -gogc -1 -memlimit 200      # sin GC por porcentaje, solo por límite de memoria
//	GODEBUG=gctrace=1 go run . -dur 1s   # una línea por ciclo de GC
//	./comparar.sh                         # tabla comparativa (incluye Green Tea frente a GC clásico)
package main

import (
	"flag"
	"fmt"
	"math/rand/v2"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
)

// nodo tiene un puntero: el GC debe recorrerlo (los objetos sin punteros son baratos).
type nodo struct {
	sig *nodo
	v   [4]int64
}

func main() {
	gogc := flag.Int("gogc", 100, "GOGC (-1 = desactivar el GC por porcentaje)")
	memlimit := flag.Int64("memlimit", 0, "GOMEMLIMIT en MiB (0 = sin límite)")
	workers := flag.Int("workers", runtime.NumCPU(), "goroutines")
	vivos := flag.Int("vivos", 150_000, "objetos vivos por worker (el conjunto de datos residente)")
	dur := flag.Duration("dur", 2*time.Second, "duración")
	flag.Parse()

	debug.SetGCPercent(*gogc)
	if *memlimit > 0 {
		debug.SetMemoryLimit(*memlimit << 20)
	}

	var ops atomic.Int64
	var maxHeap atomic.Uint64
	parar := make(chan struct{})
	var wg sync.WaitGroup

	// Muestreador del heap en uso (pico).
	wg.Go(func() {
		t := time.NewTicker(5 * time.Millisecond)
		defer t.Stop()
		var m runtime.MemStats
		for {
			select {
			case <-t.C:
				runtime.ReadMemStats(&m)
				maxHeap.Store(max(maxHeap.Load(), m.HeapInuse))
			case <-parar:
				return
			}
		}
	})

	var trabajadores sync.WaitGroup
	for w := range *workers {
		trabajadores.Go(func() {
			r := rand.New(rand.NewPCG(uint64(w), 1))
			residente := make([]*nodo, *vivos) // conjunto de datos vivo de este worker
			for i := range residente {
				residente[i] = &nodo{}
			}
			var n int64
			for {
				select {
				case <-parar:
					ops.Add(n)
					return
				default:
				}
				// basura: una cadena corta de objetos que muere enseguida
				var cabeza *nodo
				for range 40 {
					cabeza = &nodo{sig: cabeza}
				}
				_ = cabeza // (la cadena no se guarda en ningún sitio: es basura)
				// y se renueva UN objeto residente: el conjunto vivo se mantiene constante
				residente[r.IntN(len(residente))] = &nodo{}
				n += 41
			}
		})
	}

	ini := time.Now()
	time.Sleep(*dur)
	close(parar)
	trabajadores.Wait()
	wg.Wait()
	real := time.Since(ini)

	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	fmt.Printf("gogc=%-4d memlimit=%-4d | %7.1f M obj/s | ciclos GC=%-4d | GC usa %5.1f%% de la CPU | pausa total=%-8v | pico heap=%4d MiB\n",
		*gogc, *memlimit, float64(ops.Load())/real.Seconds()/1e6, ms.NumGC, ms.GCCPUFraction*100,
		time.Duration(ms.PauseTotalNs).Round(time.Microsecond), maxHeap.Load()>>20)
}
