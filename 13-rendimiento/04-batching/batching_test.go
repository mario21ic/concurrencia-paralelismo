package main

import "testing"

func conTamano(t *testing.T, n int) {
	t.Helper()
	anterior := elementos
	elementos = n
	t.Cleanup(func() { elementos = anterior })
}

// porLotes y conMutex hacen panic si la suma/total no coincide: aquí se comprueba para
// tamaños de lote que no dividen exactamente al número de elementos.
func TestPorLotesNoPierdeNiDuplica(t *testing.T) {
	conTamano(t, 100_003) // primo: ningún lote divide exactamente
	for _, tam := range []int{1, 2, 7, 64, 1000, 200_000} {
		for _, reciclar := range []bool{false, true} {
			porLotes(tam, reciclar) // hace panic si la suma es incorrecta
		}
	}
}

func TestConMutexTotalExacto(t *testing.T) {
	conTamano(t, 100_000)
	for _, w := range []int{1, 3, 8} {
		for _, tam := range []int{1, 7, 64, 1024} {
			conMutex(w, tam) // hace panic si el total es incorrecto
		}
	}
}
