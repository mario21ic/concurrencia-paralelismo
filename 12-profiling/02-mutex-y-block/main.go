// Ejercicio 2: perfiles de contención (mutex) y de bloqueo (block).
//
//	go run . -escenario=global     -mutex m.pprof -block b.pprof
//	go run . -escenario=fragmentado -mutex m.pprof -block b.pprof
//	go run . -escenario=canal      -block b.pprof
//	go tool pprof -top m.pprof
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"
)

const (
	operaciones = 400_000 // por worker
	claves      = 1024
	fragmentos  = 64
)

// trabajo simula lo que se hace con el lock tomado (unas decenas de ns).
// Devuelve un valor que el llamador USA dentro de su sección crítica: así el
// compilador no puede eliminarlo y no hay estado global compartido.
func trabajo() int {
	x := 0
	for i := 0; i < 60; i++ {
		x += i
	}
	return x
}

// ---- Un solo mutex para todo el mapa ----

type AlmacenGlobal struct {
	mu sync.Mutex
	m  map[int]int
}

func (a *AlmacenGlobal) Incrementar(k int) {
	a.mu.Lock()
	a.m[k] += 1 + trabajo()&0
	a.mu.Unlock()
}

// ---- Un mutex por fragmento (sharding) ----

type fragmento struct {
	mu sync.Mutex
	m  map[int]int
	_  [40]byte // relleno: evita que dos fragmentos compartan línea de caché
}

type AlmacenFragmentado struct{ f [fragmentos]fragmento }

func NuevoFragmentado() *AlmacenFragmentado {
	a := &AlmacenFragmentado{}
	for i := range a.f {
		a.f[i].m = map[int]int{}
	}
	return a
}

func (a *AlmacenFragmentado) Incrementar(k int) {
	f := &a.f[k%fragmentos]
	f.mu.Lock()
	f.m[k] += 1 + trabajo()&0
	f.mu.Unlock()
}

type almacen interface{ Incrementar(int) }

func correr(a almacen, workers int) time.Duration {
	var wg sync.WaitGroup
	ini := time.Now()
	for w := range workers {
		wg.Go(func() {
			x := uint32(w*2654435761 + 1)
			for range operaciones {
				x = x*1664525 + 1013904223 // generador congruencial: sin contención en rand
				a.Incrementar(int(x>>8) % claves)
			}
		})
	}
	wg.Wait()
	return time.Since(ini)
}

// canal: un productor rápido y un consumidor lento => el productor se bloquea enviando.
func canal() time.Duration {
	ch := make(chan int)
	ini := time.Now()
	var wg sync.WaitGroup
	wg.Go(func() {
		defer close(ch)
		for i := range 300 {
			ch <- i // se bloquea hasta que el consumidor llegue
		}
	})
	wg.Go(func() {
		for range ch {
			time.Sleep(200 * time.Microsecond) // consumidor lento
		}
	})
	wg.Wait()
	return time.Since(ini)
}

func volcar(nombre, archivo string) {
	if archivo == "" {
		return
	}
	f, err := os.Create(archivo)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	pprof.Lookup(nombre).WriteTo(f, 0)
}

func main() {
	escenario := flag.String("escenario", "global", "global|fragmentado|canal")
	fm := flag.String("mutex", "", "archivo del perfil de mutex")
	fb := flag.String("block", "", "archivo del perfil de bloqueo")
	flag.Parse()

	// Ambos perfiles están DESACTIVADOS por defecto. Fracción/tasa 1 = registrar todo
	// (útil para aprender; en producción usa valores como 100 o 1000 para reducir el coste).
	runtime.SetMutexProfileFraction(1)
	runtime.SetBlockProfileRate(1)

	workers := runtime.NumCPU()
	var d time.Duration
	switch *escenario {
	case "global":
		d = correr(&AlmacenGlobal{m: map[int]int{}}, workers)
	case "fragmentado":
		d = correr(NuevoFragmentado(), workers)
	case "canal":
		d = canal()
	default:
		flag.Usage()
		return
	}
	fmt.Printf("escenario=%s workers=%d tiempo=%v\n", *escenario, workers, d.Round(time.Millisecond))

	volcar("mutex", *fm)
	volcar("block", *fb)
}
