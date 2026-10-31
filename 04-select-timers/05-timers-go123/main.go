// Ejercicio 5: cambios de timers desde Go 1.23.
//
// Requiere "go 1.23" o superior en go.mod (asynctimerchan=0).
package main

import (
	"fmt"
	"runtime"
	"time"
)

func heapMB() float64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return float64(m.HeapAlloc) / 1e6
}

func main() {
	// 1) El channel del timer es síncrono: cap=0 y len=0.
	t := time.NewTimer(10 * time.Millisecond)
	time.Sleep(30 * time.Millisecond) // el timer ya expiró
	fmt.Printf("cap(t.C)=%d len(t.C)=%d\n", cap(t.C), len(t.C))

	// 2) Reset/Stop no dejan valores viejos: tras Reset NO se recibe el tick anterior.
	//    (Antes de 1.23 había que drenar t.C a mano para evitarlo.)
	ini := time.Now()
	t.Reset(100 * time.Millisecond)
	<-t.C
	fmt.Printf("tras Reset(100ms) esperó %v (no recibió el tick viejo)\n", time.Since(ini).Round(10*time.Millisecond))

	// 3) Stop garantiza que no llegará un valor viejo después.
	t2 := time.NewTimer(10 * time.Millisecond)
	time.Sleep(30 * time.Millisecond)
	fmt.Println("Stop devuelve (expiró y nadie leyó):", t2.Stop())
	select {
	case <-t2.C:
		fmt.Println("valor viejo recibido (comportamiento pre-1.23)")
	default:
		fmt.Println("no hay valor viejo en t2.C")
	}

	// 4) Timers sin referencia son recolectados aunque no hayan expirado ni se hayan parado.
	base := heapMB()
	for range 1_000_000 {
		_ = time.NewTimer(time.Hour) // sin Stop y sin guardar
	}
	fmt.Printf("heap antes: %.1f MB, tras 1M timers sin Stop + GC: %.1f MB\n", base, heapMB())
}
