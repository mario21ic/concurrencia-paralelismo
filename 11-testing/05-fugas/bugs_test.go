//go:build bugs

package fugas

import (
	"testing"
	"time"

	"go.uber.org/goleak"
)

// Estos tests FALLAN a propósito: el detector señala la fuga.

func TestObtenerMal_Goleak(t *testing.T) {
	defer goleak.VerifyNone(t)
	ObtenerMal(func() int { time.Sleep(50 * time.Millisecond); return 1 }, time.Millisecond)
}

func TestObtenerMal_Helper(t *testing.T) {
	SinFugas(t)
	ObtenerMal(func() int { time.Sleep(50 * time.Millisecond); return 1 }, time.Millisecond)
}

func TestEscuchaMal_OlvidaCerrar(t *testing.T) {
	defer goleak.VerifyNone(t)
	ch := make(chan int)
	EscuchaMal(ch, func(int) {})
	ch <- 1
	// falta close(ch)
}
