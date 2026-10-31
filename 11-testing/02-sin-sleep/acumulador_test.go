package lotes

import (
	"slices"
	"testing"
	"time"
)

// recibir espera un lote. El plazo es solo una RED DE SEGURIDAD para que un bug
// falle en lugar de colgar el test: nunca se usa para sincronizar.
func recibir(t *testing.T, ch <-chan []int) []int {
	t.Helper()
	select {
	case l := <-ch:
		return l
	case <-time.After(2 * time.Second):
		t.Fatal("no se recibió ningún lote")
		return nil
	}
}

// noRecibir comprueba que NO hay lote disponible ahora mismo.
func noRecibir(t *testing.T, ch <-chan []int) {
	t.Helper()
	select {
	case l := <-ch:
		t.Fatalf("lote inesperado: %v", l)
	default:
	}
}

func nuevo(t *testing.T, tam int) (*Acumulador, chan time.Time, <-chan []int) {
	t.Helper()
	tick := make(chan time.Time)
	lotes := make(chan []int, 10)
	a := Nuevo(tam, tick, func(l []int) { lotes <- l })
	t.Cleanup(func() { a.Cerrar() })
	return a, tick, lotes
}

func TestVuelcaPorTamano(t *testing.T) {
	a, _, lotes := nuevo(t, 3)
	a.Agregar(1)
	a.Agregar(2)
	noRecibir(t, lotes) // 2 elementos: aún no hay lote
	a.Agregar(3)
	if got := recibir(t, lotes); !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("lote = %v", got)
	}
}

// El "tiempo" lo controla el test: enviar por tick = que pasó el intervalo.
func TestVuelcaPorTick(t *testing.T) {
	a, tick, lotes := nuevo(t, 100)
	a.Agregar(1)
	a.Agregar(2)
	noRecibir(t, lotes)
	tick <- time.Time{} // sin sleeps: el bucle ya procesó los dos Agregar antes de leer el tick
	if got := recibir(t, lotes); !slices.Equal(got, []int{1, 2}) {
		t.Fatalf("lote = %v", got)
	}
}

func TestTickSinDatosNoVuelca(t *testing.T) {
	_, tick, lotes := nuevo(t, 100)
	tick <- time.Time{}
	tick <- time.Time{}
	noRecibir(t, lotes)
}

func TestCerrarVuelcaLoPendiente(t *testing.T) {
	tick := make(chan time.Time)
	lotes := make(chan []int, 1)
	a := Nuevo(100, tick, func(l []int) { lotes <- l })
	a.Agregar(7)
	a.Cerrar() // Cerrar espera a la goroutine: al volver, el volcado ya ocurrió
	if got := recibir(t, lotes); !slices.Equal(got, []int{7}) {
		t.Fatalf("lote = %v", got)
	}
}

func TestMuchosElementosNoSePierden(t *testing.T) {
	a, _, lotes := nuevo(t, 4)
	for i := range 40 {
		a.Agregar(i)
	}
	var todo []int
	for range 10 {
		todo = append(todo, recibir(t, lotes)...)
	}
	for i, v := range todo {
		if v != i {
			t.Fatalf("todo[%d] = %d", i, v)
		}
	}
}
