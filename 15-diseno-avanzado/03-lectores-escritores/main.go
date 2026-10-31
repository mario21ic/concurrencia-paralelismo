// Ejercicio 3: lectores–escritores — tres políticas de preferencia y sync.RWMutex.
//
//	go run .
package main

import (
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

type metricas struct {
	lecturas, escrituras  int64
	esperaLect, esperaEsc []time.Duration
}

// ejecutar lanza `lect` lectores y `esc` escritores pidiendo el cerrojo en bucle durante dur.
func ejecutar(l RW, lect, esc int, dur, seccion time.Duration) metricas {
	var parar atomic.Bool
	var wg sync.WaitGroup
	var m metricas
	var mu sync.Mutex
	var lecturas, escrituras atomic.Int64
	for range lect {
		wg.Go(func() {
			var esperas []time.Duration
			for !parar.Load() {
				t0 := time.Now()
				l.RLock()
				esperas = append(esperas, time.Since(t0))
				time.Sleep(seccion)
				l.RUnlock()
				lecturas.Add(1)
			}
			mu.Lock()
			m.esperaLect = append(m.esperaLect, esperas...)
			mu.Unlock()
		})
	}
	time.Sleep(seccion) // los lectores ya están en marcha
	for range esc {
		wg.Go(func() {
			var esperas []time.Duration
			for !parar.Load() {
				t0 := time.Now()
				l.Lock()
				esperas = append(esperas, time.Since(t0))
				time.Sleep(seccion)
				l.Unlock()
				escrituras.Add(1)
				time.Sleep(seccion)
			}
			mu.Lock()
			m.esperaEsc = append(m.esperaEsc, esperas...)
			mu.Unlock()
		})
	}
	time.Sleep(dur)
	parar.Store(true)
	wg.Wait()
	m.lecturas, m.escrituras = lecturas.Load(), escrituras.Load()
	return m
}

func maxD(d []time.Duration) time.Duration {
	if len(d) == 0 {
		return 0
	}
	return slices.Max(d)
}

func main() {
	const seccion = 200 * time.Microsecond
	const dur = 600 * time.Millisecond
	fmt.Printf("8 lectores en bucle + 2 escritores, secciones de %v, %v por política\n\n", seccion, dur)
	fmt.Printf("  %-34s %9s %10s %16s %16s\n", "política", "lecturas", "escrituras", "máx. espera lect", "máx. espera esc")
	for _, c := range []struct {
		nombre string
		l      RW
	}{
		{"preferencia de lectores", new(PrefLectores)},
		{"preferencia de escritores", NuevoPrefEscritores()},
		{"justa (torniquete)", new(Justo)},
		{"sync.RWMutex", new(Estandar)},
	} {
		m := ejecutar(c.l, 8, 2, dur, seccion)
		fmt.Printf("  %-34s %9d %10d %16v %16v\n", c.nombre, m.lecturas, m.escrituras,
			maxD(m.esperaLect).Round(10*time.Microsecond), maxD(m.esperaEsc).Round(10*time.Microsecond))
	}

	fmt.Println("\n2 lectores + 8 escritores en bucle (el caso inverso):")
	fmt.Printf("  %-34s %9s %10s %16s %16s\n", "política", "lecturas", "escrituras", "máx. espera lect", "máx. espera esc")
	for _, c := range []struct {
		nombre string
		l      RW
	}{
		{"preferencia de escritores", NuevoPrefEscritores()},
		{"justa (torniquete)", new(Justo)},
		{"sync.RWMutex", new(Estandar)},
	} {
		m := ejecutar(c.l, 2, 8, dur, seccion)
		fmt.Printf("  %-34s %9d %10d %16v %16v\n", c.nombre, m.lecturas, m.escrituras,
			maxD(m.esperaLect).Round(10*time.Microsecond), maxD(m.esperaEsc).Round(10*time.Microsecond))
	}
}
