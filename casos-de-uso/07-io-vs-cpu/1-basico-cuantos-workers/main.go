// Caso 7.1 (básico): ¿cuántas goroutines pongo? Depende de si el trabajo ESPERA o CALCULA.
//
// Problema: tengo 256 tareas y un pool de N workers. ¿Qué N elijo?
//   - Tarea de E/S (red, disco, BD): pasa casi todo el tiempo ESPERANDO. Mientras una goroutine espera,
//     otras pueden usar la CPU → conviene MUCHAS (cientos, miles). El límite lo pone el recurso remoto.
//   - Tarea de CPU (hash, compresión, cálculo): pasa todo el tiempo CALCULANDO. Más goroutines que
//     núcleos no aceleran nada (solo compiten) → conviene ≈ GOMAXPROCS.
//
// Este experimento lo mide: mismo pool, dos tipos de tarea, N = 1…256.
//
//	go run .
package main

import (
	"crypto/sha256"
	"fmt"
	"runtime"
	"sync"
	"time"
)

const tarea = 10 * time.Millisecond // duración objetivo de cada tarea

// EsperaES simula una llamada de red de `tarea`: la goroutine se aparca, no consume CPU.
func EsperaES() { time.Sleep(tarea) }

// rondasCPU se calibra al inicio: cuántas vueltas de SHA-256 ocupan ≈ `tarea` de CPU en esta máquina.
var rondasCPU = calibrar()

func calibrar() int {
	const prueba = 200_000
	Hash(prueba) // calentamiento
	t0 := time.Now()
	Hash(prueba)
	return int(float64(prueba) * float64(tarea) / float64(time.Since(t0)))
}

// Hash es CPU pura y determinista.
func Hash(rondas int) [32]byte {
	h := sha256.Sum256([]byte("semilla"))
	for range rondas {
		h = sha256.Sum256(h[:])
	}
	return h
}

// Ejecutar reparte `tareas` entre `workers` goroutines que toman trabajo de un channel.
func Ejecutar(tareas, workers int, f func()) time.Duration {
	cola := make(chan struct{}, tareas)
	for range tareas {
		cola <- struct{}{}
	}
	close(cola)
	var wg sync.WaitGroup
	t0 := time.Now()
	for range workers {
		wg.Go(func() {
			for range cola {
				f()
			}
		})
	}
	wg.Wait()
	return time.Since(t0)
}

func main() {
	const tareas = 256
	fmt.Printf("GOMAXPROCS=%d | %d tareas de ≈%v | rendimiento en tareas/s (mejor de 3; más es mejor)\n\n", runtime.GOMAXPROCS(0), tareas, tarea)
	fmt.Printf("%-9s %14s %14s\n", "workers", "E/S (espera)", "CPU (cálculo)")
	for _, w := range []int{1, 2, 4, 8, 16, 64, 256} {
		io := min(Ejecutar(tareas, w, EsperaES), Ejecutar(tareas, w, EsperaES), Ejecutar(tareas, w, EsperaES))
		cpuTarea := func() { Hash(rondasCPU) }
		cpu := min(Ejecutar(tareas, w, cpuTarea), Ejecutar(tareas, w, cpuTarea), Ejecutar(tareas, w, cpuTarea))
		fmt.Printf("%-9d %14.0f %14.0f\n", w, tareas/io.Seconds(), tareas/cpu.Seconds())
	}
	fmt.Println("\nE/S: sigue subiendo con N (el cuello de botella sería el recurso remoto). CPU: se estanca al llegar a GOMAXPROCS.")
}
