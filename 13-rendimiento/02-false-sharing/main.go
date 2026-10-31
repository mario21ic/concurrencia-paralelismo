// Ejercicio 2: false sharing — la contención invisible.
//
// Cada goroutine incrementa SU PROPIO contador. No hay datos compartidos ni locks,
// y aun así el rendimiento depende de lo cerca que estén los contadores en memoria.
//
//	go run .
package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/cpu"
)

const iteraciones = 150_000_000

// Contadores guarda N contadores de 8 bytes separados `salto` bytes entre sí, alineados
// al inicio de una línea de caché de 256 bytes.
type Contadores struct {
	buf   []byte
	base  unsafe.Pointer
	salto uintptr
}

func Nuevo(n int, salto uintptr) *Contadores {
	buf := make([]byte, uintptr(n)*salto+256)
	p := uintptr(unsafe.Pointer(&buf[0]))
	desplazamiento := (256 - p%256) % 256
	return &Contadores{buf: buf, base: unsafe.Add(unsafe.Pointer(&buf[0]), desplazamiento), salto: salto}
}

func (c *Contadores) ptr(i int) *uint64 {
	return (*uint64)(unsafe.Add(c.base, uintptr(i)*c.salto))
}

// Correr hace que cada una de n goroutines incremente su contador `iter` veces.
func (c *Contadores) Correr(n int, iter int) time.Duration {
	var wg sync.WaitGroup
	ini := time.Now()
	for i := range n {
		p := c.ptr(i)
		wg.Go(func() {
			for range iter {
				*p++ // lectura-modificación-escritura sobre memoria: no se puede dejar en un registro
			}
		})
	}
	wg.Wait()
	return time.Since(ini)
}

func mejor(n int, f func() time.Duration) time.Duration {
	m := time.Duration(1 << 62)
	for range n {
		m = min(m, f())
	}
	return m
}

// ---- Variante "natural": una estructura con dos campos usados por goroutines distintas ----

type EstadisticasJuntas struct {
	Peticiones uint64 // escribe la goroutine A
	Errores    uint64 // escribe la goroutine B
}

type EstadisticasSeparadas struct {
	Peticiones uint64
	_          cpu.CacheLinePad // relleno del tamaño de línea de caché de esta CPU
	Errores    uint64
}

func dosCampos(a, b *uint64) time.Duration {
	var wg sync.WaitGroup
	ini := time.Now()
	wg.Go(func() {
		for range iteraciones {
			*a++
		}
	})
	wg.Go(func() {
		for range iteraciones {
			*b++
		}
	})
	wg.Wait()
	return time.Since(ini)
}

func main() {
	n := runtime.NumCPU()
	fmt.Printf("NumCPU=%d | tamaño de línea de caché según x/sys/cpu: %d bytes\n\n", n, unsafe.Sizeof(cpu.CacheLinePad{}))

	fmt.Printf("== %d goroutines, cada una con SU contador (%d M incrementos cada una) ==\n", n, iteraciones/1_000_000)
	fmt.Printf("%12s %12s %14s\n", "separación", "tiempo", "vs. la mejor")
	saltos := []uintptr{8, 16, 32, 64, 128, 256}
	tiempos := make([]time.Duration, len(saltos))
	for i, s := range saltos {
		c := Nuevo(n, s)
		tiempos[i] = mejor(3, func() time.Duration { return c.Correr(n, iteraciones) })
	}
	mejorT := tiempos[0]
	for _, t := range tiempos {
		mejorT = min(mejorT, t)
	}
	for i, s := range saltos {
		fmt.Printf("%9d B %12v %13.1fx\n", s, tiempos[i].Round(time.Millisecond), float64(tiempos[i])/float64(mejorT))
	}

	fmt.Println("\n== dos campos de una estructura, escritos por dos goroutines ==")
	var j EstadisticasJuntas
	var s EstadisticasSeparadas
	tj := mejor(3, func() time.Duration { return dosCampos(&j.Peticiones, &j.Errores) })
	ts := mejor(3, func() time.Duration { return dosCampos(&s.Peticiones, &s.Errores) })
	fmt.Printf("juntos   (%2d bytes): %v\n", unsafe.Sizeof(j), tj.Round(time.Millisecond))
	fmt.Printf("separados(%3d bytes): %v   (%.1fx más rápido)\n", unsafe.Sizeof(s), ts.Round(time.Millisecond), float64(tj)/float64(ts))
}
