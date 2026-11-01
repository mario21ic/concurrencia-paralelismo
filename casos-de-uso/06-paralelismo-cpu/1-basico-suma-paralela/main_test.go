package main

import "testing"

func TestSumaParalelaIgualQueSecuencial(t *testing.T) {
	for _, n := range []int{0, 1, 2, 7, 1000, 12345} {
		xs := make([]int64, n)
		for i := range xs {
			xs[i] = int64(i*i%13) - 5
		}
		want := SumaSecuencial(xs)
		for _, k := range []int{1, 2, 3, 8, 100} {
			if got := SumaParalela(xs, k); got != want {
				t.Errorf("n=%d k=%d: %d, esperado %d", n, k, got, want)
			}
		}
	}
}
