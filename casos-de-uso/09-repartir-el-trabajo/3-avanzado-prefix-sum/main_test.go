package main

import (
	"slices"
	"testing"
)

func TestEscaneoSumaIgualQueSecuencial(t *testing.T) {
	for _, n := range []int{0, 1, 2, 3, 10, 1001, 65537} {
		xs := make([]int64, n)
		for i := range xs {
			xs[i] = int64(i*7%11) - 5
		}
		want := make([]int64, n)
		EscaneoSecuencial(xs, want, Suma)
		for _, k := range []int{1, 2, 3, 7, 8, 5000} {
			got := make([]int64, n)
			EscaneoParalelo(xs, got, k, Suma)
			if !slices.Equal(got, want) {
				t.Errorf("n=%d k=%d: distinto", n, k)
			}
		}
	}
}

func TestEscaneoEjemplo(t *testing.T) {
	got := make([]int64, 8)
	EscaneoParalelo([]int64{3, 1, 4, 1, 5, 9, 2, 6}, got, 3, Suma)
	if want := []int64{3, 4, 8, 9, 14, 23, 25, 31}; !slices.Equal(got, want) {
		t.Fatalf("%v, esperado %v", got, want)
	}
}

// Con una operación NO conmutativa, aplicar el desplazamiento por el lado equivocado daría resultados
// distintos: este test lo detectaría.
func TestEscaneoDeMatricesNoConmutativas(t *testing.T) {
	const n = 5000
	xs := make([]Mat, n)
	for i := range xs {
		// matrices distintas entre sí, para que el orden de los factores importe
		xs[i] = Mat{uint64(i%5 + 1), uint64(i % 3), uint64(i % 7), uint64(i%2 + 1)}
	}
	want := make([]Mat, n)
	EscaneoSecuencial(xs, want, MulMat)
	for _, k := range []int{2, 3, 8, 64} {
		got := make([]Mat, n)
		EscaneoParalelo(xs, got, k, MulMat)
		if !slices.Equal(got, want) {
			t.Errorf("k=%d: el escaneo paralelo no coincide con el secuencial", k)
		}
	}
}

func TestFibonacciConElEscaneo(t *testing.T) {
	// El prefijo i de la matriz [[1,1],[1,0]] es M^(i+1) = [[F(i+2), F(i+1)], [F(i+1), F(i)]].
	const n = 1000
	xs := make([]Mat, n)
	for i := range xs {
		xs[i] = Mat{1, 1, 1, 0}
	}
	got := make([]Mat, n)
	EscaneoParalelo(xs, got, 8, MulMat)
	fib := make([]uint64, n+3) // F(0)=0, F(1)=1
	fib[1] = 1
	for i := 2; i < len(fib); i++ {
		fib[i] = (fib[i-1] + fib[i-2]) % p
	}
	for i, m := range got {
		if m.b != fib[i+1] || m.a != fib[i+2] {
			t.Fatalf("posición %d: %+v, esperado F(%d)=%d y F(%d)=%d", i, m, i+1, fib[i+1], i+2, fib[i+2])
		}
	}
}
