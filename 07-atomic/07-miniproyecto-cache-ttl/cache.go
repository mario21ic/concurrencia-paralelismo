// Package main: caché en memoria, segura para concurrencia, con TTL por entrada
// y expiración en segundo plano ligada a un context.Context.
package main

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type entrada[V any] struct {
	valor V
	exp   time.Time // cero = no expira
}

func (e entrada[V]) expirada(ahora time.Time) bool {
	return !e.exp.IsZero() && !ahora.Before(e.exp)
}

// Estadisticas es una foto de los contadores.
type Estadisticas struct {
	Aciertos  int64
	Fallos    int64
	Expiradas int64 // entradas eliminadas por TTL (perezosa o por barrido)
}

// Cache es un mapa con TTL. El mutex protege el map (y su relación con los
// TTL); los contadores son atómicos para poder leerlos sin tomar el lock y
// sin alargar la sección crítica.
type Cache[K comparable, V any] struct {
	mu sync.Mutex
	m  map[K]entrada[V]

	ahora func() time.Time // inyectable: tests deterministas sin dormir

	aciertos, fallos, expiradas atomic.Int64

	hecho chan struct{} // se cierra cuando el barredor termina
}

// Nueva crea la caché. Si intervalo > 0, lanza una goroutine que elimina las
// entradas vencidas cada `intervalo` y que termina cuando ctx se cancela.
func Nueva[K comparable, V any](ctx context.Context, intervalo time.Duration) *Cache[K, V] {
	c := &Cache[K, V]{
		m:     make(map[K]entrada[V]),
		ahora: time.Now,
		hecho: make(chan struct{}),
	}
	if intervalo <= 0 {
		close(c.hecho)
		return c
	}
	go c.barredor(ctx, intervalo)
	return c
}

func (c *Cache[K, V]) barredor(ctx context.Context, intervalo time.Duration) {
	defer close(c.hecho)
	t := time.NewTicker(intervalo)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			c.Barrer()
		case <-ctx.Done():
			return
		}
	}
}

// Hecho devuelve un channel que se cierra cuando el barredor ha terminado.
func (c *Cache[K, V]) Hecho() <-chan struct{} { return c.hecho }

// Set guarda k con la vida ttl. ttl <= 0 significa "no expira".
func (c *Cache[K, V]) Set(k K, v V, ttl time.Duration) {
	e := entrada[V]{valor: v}
	c.mu.Lock()
	defer c.mu.Unlock()
	if ttl > 0 {
		e.exp = c.ahora().Add(ttl)
	}
	c.m[k] = e
}

// Get devuelve el valor si existe y no ha vencido. Una entrada vencida se
// elimina en el acto (expiración perezosa), aunque el barredor no haya pasado.
func (c *Cache[K, V]) Get(k K) (v V, ok bool) {
	c.mu.Lock()
	e, existe := c.m[k]
	if existe && e.expirada(c.ahora()) {
		delete(c.m, k)
		existe = false
		c.expiradas.Add(1)
	}
	c.mu.Unlock()

	if !existe {
		c.fallos.Add(1)
		return v, false
	}
	c.aciertos.Add(1)
	return e.valor, true
}

// Delete elimina la clave; devuelve true si existía (aunque estuviera vencida).
func (c *Cache[K, V]) Delete(k K) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, existe := c.m[k]
	delete(c.m, k)
	return existe
}

// Len cuenta las entradas almacenadas, incluidas las vencidas que aún no se
// han barrido.
func (c *Cache[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.m)
}

// Barrer elimina las entradas vencidas y devuelve cuántas quitó.
func (c *Cache[K, V]) Barrer() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	ahora := c.ahora()
	n := 0
	for k, e := range c.m {
		if e.expirada(ahora) {
			delete(c.m, k)
			n++
		}
	}
	c.expiradas.Add(int64(n))
	return n
}

// Stats devuelve los contadores sin tomar el mutex.
func (c *Cache[K, V]) Stats() Estadisticas {
	return Estadisticas{c.aciertos.Load(), c.fallos.Load(), c.expiradas.Load()}
}
