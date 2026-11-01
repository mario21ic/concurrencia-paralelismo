package main

import "testing"

func TestTotalesCorrectos(t *testing.T) {
	const g, total = 8, 80_000
	for nombre, c := range map[string]Contador{"mutex": &conMutex{}, "atomic": &conAtomic{}, "fragmentado": nuevoFragmentado(g)} {
		Medir(c, g, total)
		if c.Total() != total {
			t.Errorf("%s: total = %d, esperado %d", nombre, c.Total(), total)
		}
	}
}
