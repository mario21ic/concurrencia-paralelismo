// Package resiliencia contiene tres componentes dependientes del tiempo que,
// con testing/synctest, se prueban en microsegundos aunque sus plazos sean de horas.
package resiliencia

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Reintentar ejecuta f hasta `intentos` veces. Entre intentos espera
// espera, 2·espera, 4·espera... (backoff exponencial) y respeta ctx.
func Reintentar(ctx context.Context, intentos int, espera time.Duration, f func(context.Context) error) error {
	var err error
	for i := range intentos {
		if err = f(ctx); err == nil {
			return nil
		}
		if i == intentos-1 {
			break
		}
		t := time.NewTimer(espera << i)
		select {
		case <-t.C:
		case <-ctx.Done():
			t.Stop()
			return fmt.Errorf("reintento %d cancelado: %w (último error: %v)", i+1, ctx.Err(), err)
		}
	}
	return fmt.Errorf("tras %d intentos: %w", intentos, err)
}

// Debouncer ejecuta la última función enviada solo cuando pasan `d` sin nuevas llamadas.
// Típico en búsquedas "al escribir", guardado automático o recarga de configuración.
type Debouncer struct {
	d  time.Duration
	mu sync.Mutex
	t  *time.Timer
}

func NuevoDebouncer(d time.Duration) *Debouncer { return &Debouncer{d: d} }

func (b *Debouncer) Llamar(f func()) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.t != nil {
		b.t.Stop()
	}
	b.t = time.AfterFunc(b.d, f)
}

// CacheTTL es una caché mínima con expiración y barrido en segundo plano.
type CacheTTL[K comparable, V any] struct {
	mu sync.Mutex
	m  map[K]entrada[V]
}

type entrada[V any] struct {
	v   V
	exp time.Time
}

// NuevaCacheTTL lanza un barredor cada `intervalo` que termina con ctx.
func NuevaCacheTTL[K comparable, V any](ctx context.Context, intervalo time.Duration) *CacheTTL[K, V] {
	c := &CacheTTL[K, V]{m: make(map[K]entrada[V])}
	go func() {
		t := time.NewTicker(intervalo)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				c.barrer()
			case <-ctx.Done():
				return
			}
		}
	}()
	return c
}

func (c *CacheTTL[K, V]) Set(k K, v V, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[k] = entrada[V]{v, time.Now().Add(ttl)}
}

func (c *CacheTTL[K, V]) Get(k K) (v V, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.m[k]
	if !ok || !time.Now().Before(e.exp) {
		return v, false
	}
	return e.v, true
}

func (c *CacheTTL[K, V]) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.m)
}

func (c *CacheTTL[K, V]) barrer() {
	c.mu.Lock()
	defer c.mu.Unlock()
	ahora := time.Now()
	for k, e := range c.m {
		if !ahora.Before(e.exp) {
			delete(c.m, k)
		}
	}
}

// ErrTransitorio se usa en los tests como fallo recuperable.
var ErrTransitorio = errors.New("fallo transitorio")
