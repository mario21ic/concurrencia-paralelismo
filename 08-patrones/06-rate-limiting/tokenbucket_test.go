package main

import (
	"context"
	"testing"
	"time"
)

type relojFalso struct{ t time.Time }

func (r *relojFalso) Now() time.Time          { return r.t }
func (r *relojFalso) Avanzar(d time.Duration) { r.t = r.t.Add(d) }

func nuevo(rate float64, burst int) (*TokenBucket, *relojFalso) {
	r := &relojFalso{t: time.Unix(0, 0)}
	b := NuevoTokenBucket(rate, burst)
	b.ahora = r.Now
	b.ultimo = r.Now()
	return b, r
}

func TestRafagaYAgotamiento(t *testing.T) {
	b, _ := nuevo(10, 3)
	for i := range 3 {
		if !b.Allow() {
			t.Fatalf("la ráfaga %d debería permitirse", i)
		}
	}
	if b.Allow() {
		t.Fatal("tras la ráfaga no debería haber tokens")
	}
}

func TestRelleno(t *testing.T) {
	b, r := nuevo(10, 3) // 1 token cada 100ms
	for range 3 {
		b.Allow()
	}
	r.Avanzar(99 * time.Millisecond)
	if b.Allow() {
		t.Fatal("a los 99ms aún no hay token")
	}
	r.Avanzar(1 * time.Millisecond)
	if !b.Allow() {
		t.Fatal("a los 100ms debería haber un token")
	}
}

func TestNuncaSuperaLaRafaga(t *testing.T) {
	b, r := nuevo(10, 3)
	r.Avanzar(time.Hour)
	n := 0
	for b.Allow() {
		n++
	}
	if n != 3 {
		t.Fatalf("tras una hora de espera hay %d tokens; el máximo es el burst (3)", n)
	}
}

func TestWaitCancelable(t *testing.T) {
	b := NuevoTokenBucket(1, 1)
	b.Allow()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := b.Wait(ctx); err == nil {
		t.Fatal("Wait debería fallar por el plazo")
	}
}
