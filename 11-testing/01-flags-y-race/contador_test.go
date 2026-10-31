package flags

import (
	"sync"
	"testing"
)

const (
	goroutines  = 50
	incrementos = 2000
)

func incrementar(inc func()) {
	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			for range incrementos {
				inc()
			}
		})
	}
	wg.Wait()
}

func TestContadorSeguro(t *testing.T) {
	var c Contador
	incrementar(c.Inc)
	if got := c.Valor(); got != goroutines*incrementos {
		t.Fatalf("Valor = %d, want %d", got, goroutines*incrementos)
	}
}

// Los subtests paralelos con su PROPIO estado son seguros y se ejecutan a la vez.
func TestParalelosIndependientes(t *testing.T) {
	for _, n := range []int{1, 10, 100} {
		t.Run("", func(t *testing.T) {
			t.Parallel()
			var c Contador
			for range n {
				c.Inc()
			}
			if c.Valor() != n {
				t.Fatalf("Valor = %d, want %d", c.Valor(), n)
			}
		})
	}
}

// Arreglo del test dependiente del orden: restaurar el estado global con Cleanup.
func TestEntornoRestaurado(t *testing.T) {
	anterior := Entorno
	t.Cleanup(func() { Entorno = anterior })
	Entorno = "test"
	if Entorno != "test" {
		t.Fatal("no se pudo cambiar el entorno")
	}
}

func TestEntornoPorDefecto(t *testing.T) {
	if Entorno != "produccion" {
		t.Fatalf("Entorno = %q, want produccion", Entorno)
	}
}
