// Ejercicio 5: los stacks de las goroutines — crecimiento, copia, reducción y límite.
//
//	go run .                    # experimentos A-D
//	go run . -modo=desborde     # recursión infinita: error fatal
package main

import (
	"flag"
	"fmt"
	"runtime"
	"runtime/debug"
	"sync"
	"time"
	"unsafe"
)

//go:noinline
func profundo(n int) int {
	var buf [64]byte // el marco ocupa ~100 bytes
	buf[n&63] = byte(n)
	if n == 0 {
		return int(buf[0])
	}
	return profundo(n-1) + int(buf[(n+1)&63])
}

func stackEnUso() uint64 {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.StackInuse
}

func mb(b uint64) string { return fmt.Sprintf("%.1f MB", float64(b)/1e6) }

func main() {
	modo := flag.String("modo", "experimentos", "experimentos|desborde")
	flag.Parse()

	if *modo == "desborde" {
		debug.SetMaxStack(20 << 20) // 20 MB en vez de 1 GB, para fallar rápido
		fmt.Println("recursión infinita con el límite de pila en 20 MB...")
		profundo(1 << 40)
		return
	}

	// ---- A) El stack se COPIA al crecer: las direcciones de las variables locales cambian ----
	fmt.Println("== A) crecer el stack = reservar uno mayor y COPIAR el contenido ==")
	done := make(chan struct{})
	go func() {
		defer close(done)
		var x int
		antes := uintptr(unsafe.Pointer(&x))
		profundo(100_000) // ~10 MB de marcos: obliga a crecer varias veces
		despues := uintptr(unsafe.Pointer(&x))
		fmt.Printf("  dirección de la variable local antes:   %#x\n", antes)
		fmt.Printf("  dirección de la variable local después: %#x   (cambió: %v)\n", despues, antes != despues)
		fmt.Println("  Go ajusta todos los punteros hacia el stack al copiarlo; por eso no se puede guardar un puntero")
		fmt.Println("  al stack en memoria que el runtime no conozca (p. ej. como uintptr) ni pasar uno a C.")
	}()
	<-done

	// ---- B) Coste del crecimiento ----
	fmt.Println("\n== B) coste de crecer: primera ejecución frente a segunda en la MISMA goroutine ==")
	const prof = 200_000 // ≈ 20 MB
	res := make(chan [3]time.Duration)
	go func() {
		t0 := time.Now()
		profundo(prof)
		primera := time.Since(t0)
		t0 = time.Now()
		profundo(prof)
		segunda := time.Since(t0)
		t0 = time.Now()
		profundo(1000) // referencia: una recursión corta, el stack ya es grande
		res <- [3]time.Duration{primera, segunda, time.Since(t0)}
	}()
	r := <-res
	fmt.Printf("  profundidad %d: 1.ª vez %v | 2.ª vez %v  (la 1.ª es %.1f× más lenta: copias sucesivas)\n",
		prof, r[0].Round(10*time.Microsecond), r[1].Round(10*time.Microsecond), float64(r[0])/float64(r[1]))

	// ---- C) Reducción de stacks durante el GC ----
	fmt.Println("\n== C) el GC REDUCE los stacks sobredimensionados (si se usa menos de 1/4) ==")
	base := stackEnUso()
	var wg sync.WaitGroup
	liberar := make(chan struct{})
	listas := make(chan struct{}, 20)
	for range 20 {
		wg.Go(func() {
			profundo(100_000) // crece a ~16 MB cada una
			listas <- struct{}{}
			<-liberar // sigue viva, pero YA NO usa casi nada de su stack
		})
	}
	for range 20 {
		<-listas
	}
	grande := stackEnUso()
	runtime.GC()
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	tras := stackEnUso()
	fmt.Printf("  stack en uso: base %s | 20 goroutines con recursión profunda %s | tras 2 GC (siguen vivas) %s\n", mb(base), mb(grande), mb(tras))
	close(liberar)
	wg.Wait()

	// ---- D) Memoria por goroutine ----
	fmt.Println("\n== D) memoria por goroutine inactiva ==")
	runtime.GC()
	b0 := stackEnUso()
	parar := make(chan struct{})
	for range 100_000 {
		wg.Go(func() { <-parar })
	}
	time.Sleep(200 * time.Millisecond)
	b1 := stackEnUso()
	fmt.Printf("  100 000 goroutines bloqueadas: %s de stack → %.0f bytes por goroutine (mínimo del runtime: 2048)\n", mb(b1-b0), float64(b1-b0)/100_000)
	close(parar)
	wg.Wait()
}
