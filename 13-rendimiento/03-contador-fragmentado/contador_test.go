package contador

import (
	"sync"
	"testing"
	"unsafe"
)

func todos() map[string]Contador {
	return map[string]Contador{
		"atomico":     &Atomico{},
		"mutex":       &ConMutex{},
		"sin-relleno": &FragmentadoSinRelleno{},
		"relleno":     &Fragmentado{},
	}
}

func TestSumaExacta(t *testing.T) {
	for nombre, c := range todos() {
		var wg sync.WaitGroup
		for g := range 16 {
			wg.Go(func() {
				for range 10_000 {
					c.Sumar(int64(g + 1))
				}
			})
		}
		wg.Wait()
		// 10_000 × (1 + 2 + ... + 16) = 10_000 × 136
		if got := c.Total(); got != 1_360_000 {
			t.Errorf("%s: Total = %d, want 1360000", nombre, got)
		}
	}
}

func TestFragmentosOcupanSuPropiaLinea(t *testing.T) {
	var c Fragmentado
	if tam := unsafe.Sizeof(c.f[0]); tam < 64 {
		t.Fatalf("cada fragmento ocupa %d bytes; se esperaba al menos una línea de caché", tam)
	}
}

// Mientras se escribe, Total() nunca debe superar lo ya sumado ni retroceder (monotonía).
func TestTotalMonotonoConEscrituras(t *testing.T) {
	var c Fragmentado
	var wg sync.WaitGroup
	parar := make(chan struct{})
	for range 4 {
		wg.Go(func() {
			for {
				select {
				case <-parar:
					return
				default:
					c.Sumar(1)
				}
			}
		})
	}
	var anterior int64
	for range 2000 {
		if t := c.Total(); t < anterior {
			close(parar)
			wg.Wait()
			panic("Total retrocedió")
		} else {
			anterior = t
		}
	}
	close(parar)
	wg.Wait()
}

func TestSumarEnExacto(t *testing.T) {
	var c Fragmentado
	var wg sync.WaitGroup
	for g := range 16 {
		wg.Go(func() {
			for range 10_000 {
				c.SumarEn(g, 1)
			}
		})
	}
	wg.Wait()
	if got := c.Total(); got != 160_000 {
		t.Fatalf("Total = %d, want 160000", got)
	}
}
