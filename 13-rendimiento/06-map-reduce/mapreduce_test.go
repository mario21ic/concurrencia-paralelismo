package main

import (
	"maps"
	"testing"
)

func equivalentes(t *testing.T, nombre string, got, want map[uint32]int) {
	t.Helper()
	if !maps.Equal(got, want) {
		t.Fatalf("%s: resultado distinto del secuencial (len got=%d, want=%d)", nombre, len(got), len(want))
	}
}

func TestTodasLasEstrategiasCoinciden(t *testing.T) {
	for _, distintas := range []uint32{10, 5000} {
		datos := generar(200_000, distintas)
		esperado := Secuencial(datos, contar, sumar)
		for _, w := range []int{1, 3, 8, 64} {
			equivalentes(t, "mutex", CompartidoConMutex(datos, w, contar, sumar), esperado)
			equivalentes(t, "merge secuencial", LocalYMergeSecuencial(datos, w, contar, sumar), esperado)
			equivalentes(t, "merge paralelo", Aplanar(LocalYMergeParalelo(datos, w, contar, sumar)), esperado)
		}
	}
}

func TestSumaTotalConservada(t *testing.T) {
	datos := generar(100_000, 777)
	total := 0
	for _, p := range LocalYMergeParalelo(datos, 5, contar, sumar) {
		for _, v := range p {
			total += v
		}
	}
	if total != len(datos) {
		t.Fatalf("suma de recuentos = %d, want %d", total, len(datos))
	}
}

func TestParticionesDisjuntas(t *testing.T) {
	datos := generar(50_000, 1000)
	vistas := map[uint32]bool{}
	for _, p := range LocalYMergeParalelo(datos, 6, contar, sumar) {
		for k := range p {
			if vistas[k] {
				t.Fatalf("la clave %d aparece en dos particiones", k)
			}
			vistas[k] = true
		}
	}
}

func TestEntradaVaciaYPequena(t *testing.T) {
	var vacio []uint32
	if len(Secuencial(vacio, contar, sumar)) != 0 || len(LocalYMergeSecuencial(vacio, 4, contar, sumar)) != 0 {
		// con entrada vacía no hay trozos: LocalYMergeSecuencial no debe entrar en pánico
		t.Fatal("se esperaba resultado vacío")
	}
	pequeno := []uint32{1, 1, 2}
	equivalentes(t, "pequeño", LocalYMergeSecuencial(pequeno, 8, contar, sumar), map[uint32]int{1: 2, 2: 1})
}

func TestTrozosCubrenTodo(t *testing.T) {
	for _, n := range []int{0, 1, 7, 100, 1001} {
		for _, k := range []int{1, 3, 8} {
			cubierto := 0
			for _, tr := range trozos(n, k) {
				cubierto += tr[1] - tr[0]
			}
			if cubierto != n {
				t.Fatalf("n=%d k=%d: cubiertos %d", n, k, cubierto)
			}
		}
	}
}
