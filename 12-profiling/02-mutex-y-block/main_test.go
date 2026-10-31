package main

import (
	"sync"
	"testing"
)

func TestAlmacenesContabilizanBien(t *testing.T) {
	for nombre, a := range map[string]almacen{
		"global":      &AlmacenGlobal{m: map[int]int{}},
		"fragmentado": NuevoFragmentado(),
	} {
		var wg sync.WaitGroup
		for range 8 {
			wg.Go(func() {
				for i := range 5000 {
					a.Incrementar(i % claves)
				}
			})
		}
		wg.Wait()
		total := 0
		switch s := a.(type) {
		case *AlmacenGlobal:
			for _, v := range s.m {
				total += v
			}
		case *AlmacenFragmentado:
			for i := range s.f {
				for _, v := range s.f[i].m {
					total += v
				}
			}
		}
		if total != 8*5000 {
			t.Errorf("%s: total=%d, want %d", nombre, total, 8*5000)
		}
	}
}
