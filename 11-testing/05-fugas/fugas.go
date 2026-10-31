// Package fugas contiene funciones con y sin fuga de goroutines para practicar
// su detección.
package fugas

import (
	"errors"
	"time"
)

var ErrPlazo = errors.New("plazo agotado")

// ObtenerMal FUGA una goroutine cuando vence el plazo: el trabajo termina más
// tarde e intenta enviar por un channel SIN buffer del que ya nadie lee.
func ObtenerMal(trabajo func() int, plazo time.Duration) (int, error) {
	ch := make(chan int)
	go func() { ch <- trabajo() }() // se queda bloqueada para siempre si hubo timeout
	select {
	case v := <-ch:
		return v, nil
	case <-time.After(plazo):
		return 0, ErrPlazo
	}
}

// ObtenerBien: el buffer de 1 permite a la goroutine enviar y terminar aunque
// nadie escuche.
func ObtenerBien(trabajo func() int, plazo time.Duration) (int, error) {
	ch := make(chan int, 1)
	go func() { ch <- trabajo() }()
	select {
	case v := <-ch:
		return v, nil
	case <-time.After(plazo):
		return 0, ErrPlazo
	}
}

// EscuchaMal lanza una goroutine que lee hasta que se cierre el channel.
// Si quien la usa olvida cerrarlo, la goroutine se queda para siempre.
func EscuchaMal(ch <-chan int, alRecibir func(int)) {
	go func() {
		for v := range ch {
			alRecibir(v)
		}
	}()
}
