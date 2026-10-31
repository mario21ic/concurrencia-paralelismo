// Ejercicio 6: sync.Pool — reutilizar objetos temporales.
// Los benchmarks están en pool_test.go: go test -bench . -benchmem
package main

import (
	"bytes"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
)

var creados atomic.Int32

var pool = sync.Pool{
	// New se llama cuando el pool está vacío.
	New: func() any {
		creados.Add(1)
		return new(bytes.Buffer)
	},
}

func formatear(id int) string {
	buf := pool.Get().(*bytes.Buffer)
	buf.Reset()         // ¡siempre dejar el objeto en estado limpio al sacarlo (o devolverlo)!
	defer pool.Put(buf) // devolver al terminar de usarlo
	fmt.Fprintf(buf, "usuario-%d", id)
	return buf.String() // String() copia: seguro devolver tras Put
}

func main() {
	// 1) Reutilización secuencial: casi no se crean objetos nuevos.
	for i := range 1000 {
		formatear(i)
	}
	fmt.Println("secuencial, 1000 usos -> objetos creados:", creados.Load())

	// 2) Concurrente: se crean ~tantos como goroutines simultáneas, no tantos como usos.
	creados.Store(0)
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Go(func() {
			for i := range 10_000 {
				formatear(g*10_000 + i)
			}
		})
	}
	wg.Wait()
	fmt.Println("8 goroutines, 80000 usos -> objetos creados:", creados.Load())

	// 3) El pool es una CACHÉ: el GC puede vaciarlo en cualquier momento.
	creados.Store(0)
	pool.Put(new(bytes.Buffer))
	runtime.GC()
	runtime.GC() // dos GC: primero pasa a la "víctima", luego se descarta
	pool.Get()
	fmt.Println("tras 2 GC, ¿Get creó uno nuevo?", creados.Load() == 1, "(no confíes en que los objetos persistan)")
}
