// Ejercicio 2: errors.Join y errores múltiples.
package main

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var ErrNoEncontrado = errors.New("no encontrado")

type ErrValidacion struct {
	Campo, Motivo string
}

func (e *ErrValidacion) Error() string { return e.Campo + ": " + e.Motivo }

type Usuario struct {
	Nombre, Email string
	Edad          int
}

// Validar acumula TODOS los problemas en lugar de parar en el primero.
func Validar(u Usuario) error {
	var errs []error
	if strings.TrimSpace(u.Nombre) == "" {
		errs = append(errs, &ErrValidacion{"nombre", "obligatorio"})
	}
	if !strings.Contains(u.Email, "@") {
		errs = append(errs, &ErrValidacion{"email", "formato inválido"})
	}
	if u.Edad < 0 || u.Edad > 150 {
		errs = append(errs, &ErrValidacion{"edad", "fuera de rango"})
	}
	return errors.Join(errs...) // nil si errs está vacío
}

// aplanar devuelve las HOJAS del árbol de errores (recorre Unwrap() error y
// Unwrap() []error, descartando los envoltorios intermedios).
func aplanar(err error) []error {
	switch e := err.(type) {
	case nil:
		return nil
	case interface{ Unwrap() []error }:
		var hojas []error
		for _, h := range e.Unwrap() {
			hojas = append(hojas, aplanar(h)...)
		}
		return hojas
	case interface{ Unwrap() error }:
		if u := e.Unwrap(); u != nil {
			return aplanar(u)
		}
	}
	return []error{err}
}

func main() {
	// 1) Join: varios errores, un solo valor.
	err := Validar(Usuario{Nombre: " ", Email: "x", Edad: 200})
	fmt.Println("errores acumulados:")
	fmt.Println(err)

	// 2) errors.Is / errors.As atraviesan el Join.
	err2 := errors.Join(fmt.Errorf("cargando perfil: %w", ErrNoEncontrado), &ErrValidacion{"id", "vacío"})
	var ev *ErrValidacion
	fmt.Println("\nIs(ErrNoEncontrado):", errors.Is(err2, ErrNoEncontrado))
	fmt.Println("As(*ErrValidacion): ", errors.As(err2, &ev), "->", ev)

	// 3) Join descarta los nil; si todo es nil, devuelve nil.
	fmt.Println("\nJoin(nil, nil) == nil:", errors.Join(nil, nil) == nil)
	fmt.Println("Join(nil, e, nil) tiene", len(aplanar(errors.Join(nil, ErrNoEncontrado, nil))), "error(es)")

	// 4) fmt.Errorf admite varios %w (Go 1.20+).
	e3 := fmt.Errorf("falló la sincronización: %w y %w", ErrNoEncontrado, &ErrValidacion{"x", "y"})
	fmt.Println("\nvarios %w -> Is:", errors.Is(e3, ErrNoEncontrado), "| hojas:", len(aplanar(e3)))

	// 5) Concurrencia: cada goroutine escribe SU posición; Join después de Wait.
	//    Sin mutex, sin perder errores, orden determinista (el de los índices).
	comprobaciones := []func() error{
		func() error { return nil },
		func() error { time.Sleep(20 * time.Millisecond); return errors.New("disco lleno") },
		func() error { return fmt.Errorf("base de datos: %w", ErrNoEncontrado) },
		func() error { time.Sleep(5 * time.Millisecond); return errors.New("sin red") },
	}
	errs := make([]error, len(comprobaciones))
	var wg sync.WaitGroup
	for i, c := range comprobaciones {
		wg.Go(func() { errs[i] = c() })
	}
	wg.Wait()
	todos := errors.Join(errs...)
	fmt.Printf("\n%d comprobaciones, %d fallaron:\n%v\n", len(comprobaciones), len(aplanar(todos)), todos)
	fmt.Println("¿alguna es ErrNoEncontrado?", errors.Is(todos, ErrNoEncontrado))
}
