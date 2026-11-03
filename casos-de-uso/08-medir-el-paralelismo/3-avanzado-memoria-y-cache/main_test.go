package main

import "testing"

func TestKernelsNoDependenDeK(t *testing.T) {
	const total = 1 << 16 // múltiplo de todos los k probados
	for _, k := range []int{1, 2, 4, 8} {
		KernelLocal(total, k)
		a := KernelEnSlice(total, k, 1)
		b := KernelEnSlice(total, k, salto)
		if a != b {
			t.Errorf("k=%d: con y sin false sharing deben dar el MISMO resultado: %x vs %x", k, a, b)
		}
	}
}

func TestKernelMemoria(t *testing.T) {
	xs := make([]uint64, 10_007)
	var want uint64
	for i := range xs {
		xs[i] = uint64(i * 3)
		want += xs[i]
	}
	for _, k := range []int{1, 2, 7, 64} {
		if got := KernelMemoria(xs, k); got != want {
			t.Errorf("k=%d: %d, esperado %d", k, got, want)
		}
	}
}
