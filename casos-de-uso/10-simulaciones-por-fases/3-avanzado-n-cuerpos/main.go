// Caso 10.3 (avanzado): simulación de N cuerpos con workers persistentes y una barrera cíclica.
//
// Problema: N partículas se atraen entre sí (gravedad). En cada paso hay DOS fases dependientes:
//
//	fase 1: calcular la aceleración de cada cuerpo (lee las posiciones de TODOS)
//	        ── barrera ── (nadie debe mover un cuerpo mientras otro aún lee posiciones viejas)
//	fase 2: actualizar velocidades y posiciones (cada worker escribe SOLO sus cuerpos)
//	        ── barrera ── (el siguiente paso necesita las posiciones nuevas de todos)
//
// Tres formas de ejecutarlo:
//
//	secuencial ......... referencia
//	goroutines por fase  lanzar k goroutines y esperar con WaitGroup, 2 veces por paso (caso 10.2)
//	workers persistentes k goroutines que viven toda la simulación y se sincronizan con una Barrera
//	                     cíclica: no se crean goroutines en el bucle, pero cada barrera cuesta.
//
// Todas dan exactamente el mismo resultado (cada cuerpo suma las fuerzas en el mismo orden).
//
//	go run .
package main

import (
	"fmt"
	"math"
	"runtime"
	"sync"
	"time"
)

const (
	dt       = 0.01
	suavizar = 0.01 // evita la división por ~0 cuando dos cuerpos se acercan mucho
)

// Cuerpos guarda el estado en "estructura de arrays" (más amable con la caché que un slice de structs).
type Cuerpos struct {
	x, y, vx, vy, m, ax, ay []float64
}

func Nuevos(n int, semilla uint32) *Cuerpos {
	c := &Cuerpos{
		x: make([]float64, n), y: make([]float64, n), vx: make([]float64, n), vy: make([]float64, n),
		m: make([]float64, n), ax: make([]float64, n), ay: make([]float64, n),
	}
	s := semilla
	sig := func() float64 { s = s*1664525 + 1013904223; return float64(s>>8) / (1 << 24) }
	for i := range n {
		c.x[i], c.y[i] = sig()*2-1, sig()*2-1
		c.vx[i], c.vy[i] = (sig()*2-1)*0.1, (sig()*2-1)*0.1
		c.m[i] = 0.5 + sig()
	}
	return c
}

func (c *Cuerpos) Clon() *Cuerpos {
	d := Nuevos(len(c.x), 0)
	copy(d.x, c.x)
	copy(d.y, c.y)
	copy(d.vx, c.vx)
	copy(d.vy, c.vy)
	copy(d.m, c.m)
	return d
}

// aceleraciones: fase 1 para los cuerpos [desde, hasta). SOLO LEE x, y, m y escribe ax, ay propios.
func (c *Cuerpos) aceleraciones(desde, hasta int) {
	for i := desde; i < hasta; i++ {
		var ax, ay float64
		for j := range c.x {
			dx, dy := c.x[j]-c.x[i], c.y[j]-c.y[i]
			d2 := dx*dx + dy*dy + suavizar
			inv := c.m[j] / (d2 * math.Sqrt(d2))
			ax += dx * inv
			ay += dy * inv
		}
		c.ax[i], c.ay[i] = ax, ay
	}
}

// mover: fase 2 para [desde, hasta). Escribe x, y, vx, vy SOLO de sus cuerpos.
func (c *Cuerpos) mover(desde, hasta int) {
	for i := desde; i < hasta; i++ {
		c.vx[i] += c.ax[i] * dt
		c.vy[i] += c.ay[i] * dt
		c.x[i] += c.vx[i] * dt
		c.y[i] += c.vy[i] * dt
	}
}

// Momento devuelve el momento lineal total. Las fuerzas son simétricas (acción y reacción), así que
// se conserva: sirve como invariante para comprobar que el reparto en workers no corrompe el cálculo.
func (c *Cuerpos) Momento() (px, py float64) {
	for i := range c.x {
		px += c.m[i] * c.vx[i]
		py += c.m[i] * c.vy[i]
	}
	return
}

func Secuencial(c *Cuerpos, pasos int) {
	n := len(c.x)
	for range pasos {
		c.aceleraciones(0, n)
		c.mover(0, n)
	}
}

// PorFase lanza y espera a k goroutines en cada fase de cada paso.
func PorFase(c *Cuerpos, pasos, k int) {
	n := len(c.x)
	k = max(1, min(k, n))
	fase := func(f func(desde, hasta int)) {
		var wg sync.WaitGroup
		for w := range k {
			wg.Go(func() { f(w*n/k, (w+1)*n/k) })
		}
		wg.Wait()
	}
	for range pasos {
		fase(c.aceleraciones)
		fase(c.mover)
	}
}

// Persistentes lanza k workers UNA vez; cada uno repite (fase 1, barrera, fase 2, barrera) `pasos` veces.
func Persistentes(c *Cuerpos, pasos, k int) {
	n := len(c.x)
	k = max(1, min(k, n))
	b := NuevaBarrera(k)
	var wg sync.WaitGroup
	for w := range k {
		wg.Go(func() {
			desde, hasta := w*n/k, (w+1)*n/k
			for range pasos {
				c.aceleraciones(desde, hasta)
				b.Esperar() // todos terminaron de LEER las posiciones viejas
				c.mover(desde, hasta)
				b.Esperar() // todos terminaron de ESCRIBIR las posiciones nuevas
			}
		})
	}
	wg.Wait()
}

func mejor(veces int, f func()) time.Duration {
	m := time.Duration(1 << 62)
	for range veces {
		t0 := time.Now()
		f()
		m = min(m, time.Since(t0))
	}
	return m
}

func main() {
	const k = 8
	fmt.Printf("GOMAXPROCS=%d, k=%d. N cuerpos, mejor de 3. Aceleración frente al secuencial.\n\n", runtime.GOMAXPROCS(0), k)
	fmt.Printf("%-8s %-8s %-14s %-24s %-24s\n", "N", "pasos", "secuencial", "goroutines por fase", "workers + barrera")
	for _, c := range []struct{ n, pasos int }{{64, 20000}, {256, 2000}, {1024, 200}, {4096, 20}} {
		orig := Nuevos(c.n, 7)
		base := mejor(3, func() { Secuencial(orig.Clon(), c.pasos) })
		dFase := mejor(3, func() { PorFase(orig.Clon(), c.pasos, k) })
		dPers := mejor(3, func() { Persistentes(orig.Clon(), c.pasos, k) })
		fmt.Printf("%-8d %-8d %-14v %-24s %-24s\n", c.n, c.pasos, base.Round(100*time.Microsecond),
			fmt.Sprintf("%v (%.1f×)", dFase.Round(100*time.Microsecond), float64(base)/float64(dFase)),
			fmt.Sprintf("%v (%.1f×)", dPers.Round(100*time.Microsecond), float64(base)/float64(dPers)))
	}
}
