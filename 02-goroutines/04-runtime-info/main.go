// Ejercicio 4: paquete runtime y crecimiento del stack.
//
//	go run .
//	GOMAXPROCS=2 go run .
package main

import (
	"fmt"
	"runtime"
	"sync"
)

// recursion usa un frame grande para forzar el crecimiento del stack.
//
//go:noinline
func recursion(n int) int {
	var buf [1024]byte
	buf[n%len(buf)] = byte(n)
	if n == 0 {
		return int(buf[0])
	}
	return recursion(n-1) + int(buf[n%len(buf)])
}

func stackInuse() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.StackInuse
}

func main() {
	fmt.Println("NumCPU:        ", runtime.NumCPU())
	fmt.Println("GOMAXPROCS(0): ", runtime.GOMAXPROCS(0)) // 0 = solo consultar
	fmt.Println("NumGoroutine:  ", runtime.NumGoroutine())
	fmt.Println("GOOS/GOARCH:   ", runtime.GOOS, runtime.GOARCH)

	// NumGoroutine antes/durante/después.
	parar := make(chan struct{})
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() { <-parar })
	}
	fmt.Println("NumGoroutine con 10 bloqueadas:", runtime.NumGoroutine())
	close(parar)
	wg.Wait()

	// Crecimiento de stack: una goroutine con recursión profunda.
	antes := stackInuse()
	listo, seguir := make(chan struct{}), make(chan struct{})
	go func() {
		recursion(2000) // ~2 MB de frames: el stack se duplica hasta 4 MB
		close(listo)
		<-seguir
	}()
	<-listo
	despues := stackInuse()
	fmt.Printf("\nStackInuse antes: %d KB, con recursión profunda: %d KB\n", antes/1024, despues/1024)
	close(seguir)
}
