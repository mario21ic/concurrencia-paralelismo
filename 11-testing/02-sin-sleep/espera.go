package lotes

import (
	"testing"
	"time"
)

// Eventualmente sondea cond hasta que se cumpla o venza el plazo.
// ÚLTIMO RECURSO: úsalo solo cuando no puedas inyectar un hook, un channel o
// un reloj. Sigue dependiendo del tiempo real (plazo y paso).
func Eventualmente(t *testing.T, plazo time.Duration, cond func() bool, msg string) {
	t.Helper()
	limite := time.Now().Add(plazo)
	for !cond() {
		if time.Now().After(limite) {
			t.Fatalf("no se cumplió en %v: %s", plazo, msg)
		}
		time.Sleep(time.Millisecond)
	}
}
