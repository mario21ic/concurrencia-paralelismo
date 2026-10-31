package main

import (
	"sync"
	"sync/atomic"
	"testing"
)

func TestBasico(t *testing.T) {
	m := NuevoFragmentado[string, int](8)
	m.Store("a", 1)
	m.Store("b", 2)
	if v, ok := m.Load("a"); !ok || v != 1 {
		t.Fatalf("Load a = %d,%v", v, ok)
	}
	m.Delete("a")
	if _, ok := m.Load("a"); ok {
		t.Fatal("a no debería existir")
	}
	if m.Len() != 1 {
		t.Fatalf("Len = %d", m.Len())
	}
}

// Load+Store NO es atómico (se pierden incrementos); Update sí.
func TestUpdateEsAtomico(t *testing.T) {
	m := NuevoFragmentado[string, int](4)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 5000 {
				m.Update("contador", func(v int, _ bool) (int, bool) { return v + 1, false })
			}
		})
	}
	wg.Wait()
	if v, _ := m.Load("contador"); v != 40000 {
		t.Fatalf("contador = %d, esperado 40000", v)
	}
}

func TestLoadStoreNoEsAtomico(t *testing.T) {
	m := NuevoFragmentado[string, int](4)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 5000 {
				v, _ := m.Load("contador") // lectura y escritura por separado: carrera lógica
				m.Store("contador", v+1)
			}
		})
	}
	wg.Wait()
	if v, _ := m.Load("contador"); v == 40000 {
		t.Skip("improbable: no se perdió ningún incremento en esta ejecución")
	}
}

func TestLoadOrStoreUnSoloGanador(t *testing.T) {
	m := NuevoFragmentado[int, int](16)
	var nuevos atomic.Int32
	var wg sync.WaitGroup
	for i := range 32 {
		wg.Go(func() {
			if _, cargado := m.LoadOrStore(7, i); !cargado {
				nuevos.Add(1)
			}
		})
	}
	wg.Wait()
	if nuevos.Load() != 1 {
		t.Fatalf("%d goroutines creyeron ser las primeras", nuevos.Load())
	}
}

func TestRangeNoInterbloqueaYVeLoQueHabia(t *testing.T) {
	m := NuevoFragmentado[int, int](8)
	for i := range 100 {
		m.Store(i, i)
	}
	originales := 0
	m.Range(func(k, v int) bool {
		if k < 1000 {
			originales++
			m.Store(k+1000, v) // modificar el mapa desde el callback no debe bloquear
		}
		return true // las claves nuevas pueden verse o no: la consistencia es por fragmento
	})
	if originales != 100 || m.Len() != 200 {
		t.Fatalf("originales=%d len=%d", originales, m.Len())
	}
}

func TestReparto(t *testing.T) {
	m := NuevoFragmentado[int, int](16)
	for i := range 16000 {
		m.Store(i, i)
	}
	for i := range m.frag {
		if n := len(m.frag[i].m); n < 700 || n > 1300 { // media 1000
			t.Fatalf("fragmento %d con %d claves: reparto muy desigual", i, n)
		}
	}
}

func TestConcurrenteConRace(t *testing.T) {
	for nombre, m := range map[string]Mapa{"global": NuevoGlobal(), "sync": new(MapaSync), "frag": NuevoFragInt(16)} {
		t.Run(nombre, func(t *testing.T) {
			var wg sync.WaitGroup
			for g := range 8 {
				wg.Go(func() {
					for i := range 2000 {
						k := (g*31 + i) % 100
						m.Store(k, i)
						m.Load(k)
						if i%7 == 0 {
							m.Delete(k)
						}
					}
				})
			}
			wg.Wait()
		})
	}
}
