package main

import (
	"math/rand/v2"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
)

func ordenadas(t *testing.T, l *Lista[int]) []int {
	t.Helper()
	var ks []int
	for k := range l.All() {
		ks = append(ks, k)
	}
	if !slices.IsSorted(ks) {
		t.Fatal("las claves no están ordenadas")
	}
	for i := 1; i < len(ks); i++ {
		if ks[i] == ks[i-1] {
			t.Fatalf("clave duplicada %d", ks[i])
		}
	}
	return ks
}

// Contra un modelo (map) con operaciones aleatorias: debe comportarse como un conjunto.
func TestSecuencialContraModelo(t *testing.T) {
	l := Nueva[int]()
	modelo := map[int]bool{}
	r := rand.New(rand.NewPCG(1, 2))
	for range 50_000 {
		k := r.IntN(500)
		switch r.IntN(3) {
		case 0:
			if got, want := l.Add(k), !modelo[k]; got != want {
				t.Fatalf("Add(%d) = %v, esperado %v", k, got, want)
			}
			modelo[k] = true
		case 1:
			if got, want := l.Remove(k), modelo[k]; got != want {
				t.Fatalf("Remove(%d) = %v, esperado %v", k, got, want)
			}
			delete(modelo, k)
		default:
			if got := l.Contains(k); got != modelo[k] {
				t.Fatalf("Contains(%d) = %v, esperado %v", k, got, modelo[k])
			}
		}
	}
	if got := len(ordenadas(t, l)); got != len(modelo) {
		t.Fatalf("tamaño %d, esperado %d", got, len(modelo))
	}
}

// Cada goroutine es DUEÑA de las claves k ≡ id (mod G) y las comprueba contra su modelo local, pero
// las claves vecinas (de otras goroutines) están en las mismas listas de punteros: estresa la
// estructura y verifica la corrección de cada clave.
func TestConcurrenteContraModeloPorGoroutine(t *testing.T) {
	const G = 8
	l := Nueva[int]()
	var wg sync.WaitGroup
	finales := make([]map[int]bool, G)
	for id := range G {
		wg.Go(func() {
			r := rand.New(rand.NewPCG(uint64(id), 9))
			modelo := map[int]bool{}
			for range 20_000 {
				k := r.IntN(300)*G + id
				switch r.IntN(3) {
				case 0:
					if got, want := l.Add(k), !modelo[k]; got != want {
						t.Errorf("g%d Add(%d) = %v, esperado %v", id, k, got, want)
						return
					}
					modelo[k] = true
				case 1:
					if got, want := l.Remove(k), modelo[k]; got != want {
						t.Errorf("g%d Remove(%d) = %v, esperado %v", id, k, got, want)
						return
					}
					delete(modelo, k)
				default:
					if got := l.Contains(k); got != modelo[k] {
						t.Errorf("g%d Contains(%d) = %v, esperado %v", id, k, got, modelo[k])
						return
					}
				}
			}
			finales[id] = modelo
		})
	}
	wg.Wait()
	if t.Failed() {
		return
	}
	var esperado []int
	for _, m := range finales {
		for k := range m {
			esperado = append(esperado, k)
		}
	}
	slices.Sort(esperado)
	if got := ordenadas(t, l); !slices.Equal(got, esperado) {
		t.Fatalf("contenido final distinto: %d claves frente a %d esperadas", len(got), len(esperado))
	}
}

// Muchas goroutines compiten por LAS MISMAS claves: por cada clave, #Add exitosos - #Remove exitosos
// debe ser 0 o 1, y coincidir con Contains al final.
func TestCompetenciaPorLasMismasClaves(t *testing.T) {
	const claves = 16
	l := Nueva[int]()
	var adds, removes [claves]atomic.Int64
	var wg sync.WaitGroup
	for id := range 8 {
		wg.Go(func() {
			r := rand.New(rand.NewPCG(uint64(id), 3))
			for range 30_000 {
				k := r.IntN(claves)
				if r.IntN(2) == 0 {
					if l.Add(k) {
						adds[k].Add(1)
					}
				} else if l.Remove(k) {
					removes[k].Add(1)
				}
			}
		})
	}
	wg.Wait()
	for k := range claves {
		neto := adds[k].Load() - removes[k].Load()
		if neto != 0 && neto != 1 {
			t.Fatalf("clave %d: altas-bajas = %d (debe ser 0 o 1)", k, neto)
		}
		if l.Contains(k) != (neto == 1) {
			t.Fatalf("clave %d: Contains inconsistente con el balance %d", k, neto)
		}
	}
	ordenadas(t, l)
}

func TestAddUnSoloGanador(t *testing.T) {
	l := Nueva[int]()
	var ganadores atomic.Int32
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			if l.Add(42) {
				ganadores.Add(1)
			}
		})
	}
	wg.Wait()
	if ganadores.Load() != 1 {
		t.Fatalf("%d goroutines insertaron la misma clave", ganadores.Load())
	}
}
