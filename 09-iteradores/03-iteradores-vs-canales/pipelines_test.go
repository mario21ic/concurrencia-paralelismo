package pipelines

import "testing"

var datos = Datos()

func TestTodosCoinciden(t *testing.T) {
	want := SumaBucle(datos)
	for nombre, got := range map[string]int{
		"iter":        SumaIter(datos),
		"pull":        SumaPull(datos),
		"canales":     SumaCanales(datos, 0),
		"canales-buf": SumaCanales(datos, 64),
	} {
		if got != want {
			t.Errorf("%s = %d, want %d", nombre, got, want)
		}
	}
}

var sink int

func BenchmarkBucle(b *testing.B) {
	for b.Loop() {
		sink = SumaBucle(datos)
	}
}

func BenchmarkIteradores(b *testing.B) {
	for b.Loop() {
		sink = SumaIter(datos)
	}
}

func BenchmarkIteradoresPull(b *testing.B) {
	for b.Loop() {
		sink = SumaPull(datos)
	}
}

func BenchmarkCanalesSinBuffer(b *testing.B) {
	for b.Loop() {
		sink = SumaCanales(datos, 0)
	}
}

func BenchmarkCanalesBuffer64(b *testing.B) {
	for b.Loop() {
		sink = SumaCanales(datos, 64)
	}
}
