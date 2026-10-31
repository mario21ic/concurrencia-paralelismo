package main

import (
	"errors"
	"fmt"
	"runtime/debug"

	"golang.org/x/sync/errgroup"
)

// PanicError convierte un pánico recuperado en un error con su traza.
type PanicError struct {
	Valor any
	Stack []byte
}

func (p *PanicError) Error() string { return fmt.Sprintf("panic: %v", p.Valor) }

// Unwrap permite errors.Is/As cuando se hizo panic(err).
func (p *PanicError) Unwrap() error {
	if e, ok := p.Valor.(error); ok {
		return e
	}
	return nil
}

// Capturar ejecuta f y convierte un pánico en *PanicError.
// recover solo funciona dentro de un defer de LA MISMA goroutine que entra en pánico.
func Capturar(f func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = &PanicError{Valor: r, Stack: debug.Stack()}
		}
	}()
	return f()
}

// GoSeguro lanza f en el grupo; si f entra en pánico, el grupo recibe un error
// (y, con WithContext, cancela al resto) en lugar de tumbar el proceso.
func GoSeguro(g *errgroup.Group, f func() error) {
	g.Go(func() error { return Capturar(f) })
}

// EsPanic informa si algún error de la cadena fue un pánico.
func EsPanic(err error) (*PanicError, bool) {
	var pe *PanicError
	return pe, errors.As(err, &pe)
}
