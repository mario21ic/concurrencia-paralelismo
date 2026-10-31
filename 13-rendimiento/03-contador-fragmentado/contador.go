// Package contador compara formas de contar de forma concurrente cuando hay MUCHA escritura.
package contador

import (
	"math/rand/v2"
	"sync"
	"sync/atomic"

	"golang.org/x/sys/cpu"
)

// Interfaz común para poder comparar.
type Contador interface {
	Sumar(n int64)
	Total() int64
}

// ---- 1) Un solo atómico: correcto, pero todos los núcleos pelean por una línea de caché ----

type Atomico struct{ v atomic.Int64 }

func (c *Atomico) Sumar(n int64) { c.v.Add(n) }
func (c *Atomico) Total() int64  { return c.v.Load() }

// ---- 2) Mutex ----

type ConMutex struct {
	mu sync.Mutex
	v  int64
}

func (c *ConMutex) Sumar(n int64) { c.mu.Lock(); c.v += n; c.mu.Unlock() }
func (c *ConMutex) Total() int64  { c.mu.Lock(); defer c.mu.Unlock(); return c.v }

// ---- 3) Fragmentado SIN relleno: varios atómicos en la misma línea de caché ----

const fragmentos = 64 // potencia de 2

type FragmentadoSinRelleno struct{ f [fragmentos]atomic.Int64 }

func (c *FragmentadoSinRelleno) Sumar(n int64) {
	c.f[rand.Uint32()&(fragmentos-1)].Add(n)
}

func (c *FragmentadoSinRelleno) Total() (t int64) {
	for i := range c.f {
		t += c.f[i].Load()
	}
	return
}

// ---- 4) Fragmentado CON relleno: cada fragmento en su propia línea de caché ----

type fragmento struct {
	v atomic.Int64
	_ cpu.CacheLinePad // tamaño de línea de caché de la CPU (128 B en Apple Silicon, 64 B en x86)
}

type Fragmentado struct{ f [fragmentos]fragmento }

// Sumar elige un fragmento al azar: los núcleos casi nunca coinciden, así que casi no hay
// contención. math/rand/v2 usa un generador POR HILO sin locks (cuesta unos pocos ns).
// Go no expone el número de P actual; elegir al azar es la alternativa portable.
func (c *Fragmentado) Sumar(n int64) {
	c.f[rand.Uint32()&(fragmentos-1)].v.Add(n)
}

// Total es O(fragmentos): leer es más caro que escribir (el compromiso del patrón).
// El resultado es una suma de lecturas no simultáneas: exacto si no hay escrituras en curso.
func (c *Fragmentado) Total() (t int64) {
	for i := range c.f {
		t += c.f[i].v.Load()
	}
	return
}

// SumarEn usa un fragmento ESTABLE elegido por el llamador (p. ej. el índice de su worker).
// Cada línea de caché la toca casi siempre el mismo núcleo, así que NO migra entre núcleos:
// es lo más cercano a un contador "por P" que se puede hacer en Go de forma portable.
func (c *Fragmentado) SumarEn(indice int, n int64) {
	c.f[uint(indice)&(fragmentos-1)].v.Add(n)
}
