// Ejercicio 1: concurrencia vs paralelismo.
//
// Ejecuta las mismas N tareas de tres formas y compara tiempos:
//  1. secuencial
//  2. concurrente con 1 solo P (GOMAXPROCS=1): intercalado, sin paralelismo
//  3. concurrente con todos los núcleos: paralelismo real
//
// Se hace con tareas CPU-bound y con tareas I/O-bound (simuladas con Sleep).
package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

const tareas = 8

// cpuTask consume CPU sin dormir.
func cpuTask() {
	x := 0
	for i := 0; i < 300_000_000; i++ {
		x += i
	}
	_ = x
}

// ioTask simula esperar una respuesta de red/disco.
func ioTask() { time.Sleep(200 * time.Millisecond) }

func secuencial(f func()) time.Duration {
	ini := time.Now()
	for range tareas {
		f()
	}
	return time.Since(ini)
}

func concurrente(f func()) time.Duration {
	ini := time.Now()
	var wg sync.WaitGroup
	for range tareas {
		wg.Go(f) // Go 1.25+
	}
	wg.Wait()
	return time.Since(ini)
}

func medir(nombre string, f func()) {
	fmt.Printf("\n== %s ==\n", nombre)
	fmt.Printf("%-40s %v\n", "secuencial:", secuencial(f).Round(time.Millisecond))

	prev := runtime.GOMAXPROCS(1)
	fmt.Printf("%-40s %v\n", "concurrente, GOMAXPROCS=1:", concurrente(f).Round(time.Millisecond))

	runtime.GOMAXPROCS(prev)
	fmt.Printf("%-40s %v\n", fmt.Sprintf("concurrente, GOMAXPROCS=%d:", prev), concurrente(f).Round(time.Millisecond))
}

func main() {
	fmt.Printf("NumCPU=%d, tareas=%d\n", runtime.NumCPU(), tareas)
	medir("CPU-bound", cpuTask)
	medir("I/O-bound", ioTask)
}
