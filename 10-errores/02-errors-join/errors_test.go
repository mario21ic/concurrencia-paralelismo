package main

import (
	"errors"
	"testing"
)

func TestValidarAcumula(t *testing.T) {
	err := Validar(Usuario{Nombre: "", Email: "x", Edad: -1})
	if n := len(aplanar(err)); n != 3 {
		t.Fatalf("errores=%d, want 3", n)
	}
	var ev *ErrValidacion
	if !errors.As(err, &ev) {
		t.Fatal("As debería encontrar *ErrValidacion")
	}
}

func TestValidarOK(t *testing.T) {
	if err := Validar(Usuario{Nombre: "Ana", Email: "a@b.c", Edad: 30}); err != nil {
		t.Fatalf("err=%v, want nil", err)
	}
}

func TestJoinNilEsNil(t *testing.T) {
	if errors.Join(nil, nil) != nil {
		t.Fatal("Join de nils debe ser nil")
	}
}

func TestIsAtravesaJoin(t *testing.T) {
	err := errors.Join(errors.New("a"), ErrNoEncontrado)
	if !errors.Is(err, ErrNoEncontrado) {
		t.Fatal("Is debe atravesar Join")
	}
}
