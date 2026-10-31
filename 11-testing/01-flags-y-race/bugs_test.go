//go:build bugs

package flags

import "testing"

// Falla (o no) según el número de CPUs: con -cpu 1 casi nunca falla.
func TestContadorMalo(t *testing.T) {
	var c ContadorMalo
	incrementar(c.Inc)
	if got := c.Valor(); got != goroutines*incrementos {
		t.Fatalf("Valor = %d, want %d (se perdieron incrementos)", got, goroutines*incrementos)
	}
}

// Pasa en el orden del archivo (B antes que A), pero falla si -shuffle
// ejecuta primero a A: A deja el estado global modificado.

func TestB_EsperaEntornoPorDefecto(t *testing.T) {
	if Entorno != "produccion" {
		t.Fatalf("Entorno = %q, want produccion (¿otro test lo dejó sucio?)", Entorno)
	}
}

func TestA_CambiaEntornoSinRestaurar(t *testing.T) {
	Entorno = "test" // ¡sin Cleanup!
}
