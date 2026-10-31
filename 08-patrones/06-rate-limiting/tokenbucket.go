package main

import (
	"context"
	"sync"
	"time"
)

// TokenBucket implementa un limitador de tasa: se rellena a `rate` tokens por
// segundo hasta un máximo de `burst`; cada operación consume un token.
//
// El relleno es PEREZOSO: se calcula al consultar según el tiempo transcurrido,
// sin goroutines ni tickers.
type TokenBucket struct {
	mu     sync.Mutex
	rate   float64 // tokens por segundo
	burst  float64
	tokens float64
	ultimo time.Time
	ahora  func() time.Time // inyectable para tests
}

func NuevoTokenBucket(rate float64, burst int) *TokenBucket {
	b := &TokenBucket{rate: rate, burst: float64(burst), tokens: float64(burst), ahora: time.Now}
	b.ultimo = b.ahora()
	return b
}

// rellenar debe llamarse con b.mu tomado.
func (b *TokenBucket) rellenar() {
	ahora := b.ahora()
	b.tokens = min(b.burst, b.tokens+ahora.Sub(b.ultimo).Seconds()*b.rate)
	b.ultimo = ahora
}

// Allow consume un token si hay; no bloquea.
func (b *TokenBucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.rellenar()
	if b.tokens >= 1 {
		b.tokens--
		return true
	}
	return false
}

// Wait bloquea hasta disponer de un token o hasta que ctx termine.
// (No garantiza orden FIFO entre varios esperando.)
func (b *TokenBucket) Wait(ctx context.Context) error {
	for {
		b.mu.Lock()
		b.rellenar()
		if b.tokens >= 1 {
			b.tokens--
			b.mu.Unlock()
			return nil
		}
		falta := time.Duration((1 - b.tokens) / b.rate * float64(time.Second))
		b.mu.Unlock()

		t := time.NewTimer(falta)
		select {
		case <-t.C:
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		}
	}
}
