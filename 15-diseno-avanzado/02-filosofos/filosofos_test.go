package main

import (
	"testing"
	"time"
)

func TestIngenuaSeInterbloqueaConPausa(t *testing.T) {
	r := Cena(Ingenua, 5, 300*time.Millisecond, time.Millisecond, 20*time.Millisecond)
	if r.Total() != 0 {
		t.Fatalf("se esperaba interbloqueo (0 comidas) y hubo %d", r.Total())
	}
}

func TestEstrategiasSeguras(t *testing.T) {
	for _, e := range []Estrategia{Ordenada, Mayordomo, Asimetrica} {
		t.Run(e.String(), func(t *testing.T) {
			r := Cena(e, 5, 400*time.Millisecond, time.Millisecond, 10*time.Millisecond)
			if r.UltimaComida < 300*time.Millisecond {
				t.Fatalf("dejó de progresar a los %v", r.UltimaComida)
			}
			for i, c := range r.Comidas {
				if c == 0 {
					t.Fatalf("el filósofo %d no comió nunca (inanición): %v", i, r.Comidas)
				}
			}
		})
	}
}

// Dos vecinos nunca comen a la vez: se comprueba con el recuento máximo de comidas simultáneas.
func TestExclusionMutuaDeTenedores(t *testing.T) {
	r := Cena(Ordenada, 5, 200*time.Millisecond, time.Millisecond, 0)
	// 5 filósofos, 5 tenedores, 2 por comida ⇒ como mucho 2 comen a la vez ⇒ en `dur`/comer hay un tope.
	tope := int64(2 * (200 * time.Millisecond / time.Millisecond))
	if r.Total() > tope {
		t.Fatalf("hubo más comidas (%d) de las posibles con 5 tenedores (%d): dos filósofos compartieron un tenedor", r.Total(), tope)
	}
}
