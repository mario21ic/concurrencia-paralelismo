// Package main: map-reduce en memoria con varias estrategias de combinación.
package main

import (
	"hash/maphash"
	"sync"
)

// Entrada → (clave, valor): la fase "map". reducir combina dos valores de la misma clave.
type (
	Mapa[E any, K comparable, V any] func(E) (K, V)
	Reducir[V any]                   func(V, V) V
)

// trozos reparte n elementos en k intervalos contiguos casi iguales.
func trozos(n, k int) [][2]int {
	out := make([][2]int, 0, k)
	for i := range k {
		ini, fin := i*n/k, (i+1)*n/k
		if ini < fin {
			out = append(out, [2]int{ini, fin})
		}
	}
	return out
}

// Secuencial es la referencia.
func Secuencial[E any, K comparable, V any](entradas []E, mapa Mapa[E, K, V], reducir Reducir[V]) map[K]V {
	res := make(map[K]V)
	for _, e := range entradas {
		k, v := mapa(e)
		if actual, ok := res[k]; ok {
			res[k] = reducir(actual, v)
		} else {
			res[k] = v
		}
	}
	return res
}

// CompartidoConMutex: el enfoque ingenuo, un mapa global y un lock por elemento.
func CompartidoConMutex[E any, K comparable, V any](entradas []E, workers int, mapa Mapa[E, K, V], reducir Reducir[V]) map[K]V {
	res := make(map[K]V)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, t := range trozos(len(entradas), workers) {
		wg.Go(func() {
			for _, e := range entradas[t[0]:t[1]] {
				k, v := mapa(e)
				mu.Lock()
				if actual, ok := res[k]; ok {
					res[k] = reducir(actual, v)
				} else {
					res[k] = v
				}
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	return res
}

// fasMap: cada worker agrega en un mapa LOCAL (sin locks). Devuelve un mapa por worker.
func fasMap[E any, K comparable, V any](entradas []E, workers int, mapa Mapa[E, K, V], reducir Reducir[V]) []map[K]V {
	ts := trozos(len(entradas), workers)
	locales := make([]map[K]V, len(ts))
	var wg sync.WaitGroup
	for i, t := range ts {
		wg.Go(func() { locales[i] = Secuencial(entradas[t[0]:t[1]], mapa, reducir) })
	}
	wg.Wait()
	return locales
}

// LocalYMergeSecuencial: fase map paralela sin locks + fusión en UNA goroutine.
// Si hay muchas claves distintas, la fusión secuencial limita la escalabilidad (Amdahl).
func LocalYMergeSecuencial[E any, K comparable, V any](entradas []E, workers int, mapa Mapa[E, K, V], reducir Reducir[V]) map[K]V {
	locales := fasMap(entradas, workers, mapa, reducir)
	if len(locales) == 0 { // entrada vacía: no se creó ningún trozo
		return map[K]V{}
	}
	res := locales[0]
	for _, m := range locales[1:] {
		for k, v := range m {
			if actual, ok := res[k]; ok {
				res[k] = reducir(actual, v)
			} else {
				res[k] = v
			}
		}
	}
	return res
}

// LocalYMergeParalelo: la fase de reducción también es paralela. Cada "reductor" r es dueño
// de las claves con hash%R == r, recorre TODOS los mapas locales y se queda solo con las suyas.
// Los reductores escriben en mapas distintos: no hay locks. Devuelve el resultado PARTICIONADO
// (R mapas con claves disjuntas); unirlos en uno solo costaría otra fusión secuencial.
func LocalYMergeParalelo[E any, K comparable, V any](entradas []E, workers int, mapa Mapa[E, K, V], reducir Reducir[V]) []map[K]V {
	locales := fasMap(entradas, workers, mapa, reducir)
	semilla := maphash.MakeSeed()
	R := workers
	partes := make([]map[K]V, R)
	var wg sync.WaitGroup
	for r := range R {
		wg.Go(func() {
			parte := make(map[K]V)
			for _, m := range locales {
				for k, v := range m {
					if int(maphash.Comparable(semilla, k)%uint64(R)) != r {
						continue
					}
					if actual, ok := parte[k]; ok {
						parte[k] = reducir(actual, v)
					} else {
						parte[k] = v
					}
				}
			}
			partes[r] = parte
		})
	}
	wg.Wait()
	return partes
}

// Aplanar une un resultado particionado en un solo mapa (para comparar en los tests).
func Aplanar[K comparable, V any](partes []map[K]V) map[K]V {
	res := make(map[K]V)
	for _, p := range partes {
		for k, v := range p {
			res[k] = v
		}
	}
	return res
}
