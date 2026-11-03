// Caso 9.3 (avanzado): un algoritmo que PARECE secuencial y se puede paralelizar — el escaneo (suma de prefijos).
//
// Problema: dado xs, calcular dst donde dst[i] = xs[0] ⊕ xs[1] ⊕ … ⊕ xs[i], para una operación ⊕
// ASOCIATIVA. Cada elemento depende del anterior, así que a primera vista no hay nada que repartir.
// Pero hay un truco de tres fases:
//
//	fase 1 (paralela)    cada worker hace el escaneo de SU bloque y guarda el total del bloque
//	fase 2 (secuencial)  escaneo de los k totales de bloque (k es pequeño: coste despreciable)
//	fase 3 (paralela)    cada worker recorre su bloque otra vez aplicando el desplazamiento que le toca
//
// Coste: el algoritmo paralelo hace ≈ el DOBLE de operaciones y el DOBLE de tráfico de memoria. Solo
// gana si el reparto compensa ese trabajo extra. El main compara dos operaciones: una suma (barata) y
// el producto de matrices 2×2 módulo p (más cara por elemento).
//
// Basta con que ⊕ sea ASOCIATIVA; no hace falta que sea conmutativa (el producto de matrices no lo
// es): por eso el desplazamiento se aplica siempre por la IZQUIERDA: dst[i] = desplazamiento ⊕ dst[i].
//
//	go run .
package main

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

// EscaneoSecuencial escribe en dst los prefijos de xs (len(dst) >= len(xs)).
func EscaneoSecuencial[T any](xs, dst []T, op func(a, b T) T) {
	for i := range xs {
		if i == 0 {
			dst[0] = xs[0]
		} else {
			dst[i] = op(dst[i-1], xs[i])
		}
	}
}

// EscaneoParalelo implementa las tres fases con k workers.
func EscaneoParalelo[T any](xs, dst []T, k int, op func(a, b T) T) {
	n := len(xs)
	if n == 0 {
		return
	}
	k = max(1, min(k, n)) // con k ≤ n, todos los bloques tienen al menos un elemento
	limites := func(w int) (int, int) { return w * n / k, (w + 1) * n / k }
	totales := make([]T, k)
	var wg sync.WaitGroup

	// Fase 1: escaneo LOCAL de cada bloque (sin tener en cuenta los anteriores).
	for w := range k {
		wg.Go(func() {
			desde, hasta := limites(w)
			EscaneoSecuencial(xs[desde:hasta], dst[desde:hasta], op)
			totales[w] = dst[hasta-1]
		})
	}
	wg.Wait() // barrera: la fase 2 necesita TODOS los totales

	// Fase 2: desplaza[w] = total de los bloques 0..w-1 (el bloque 0 no necesita corrección).
	desplaza := make([]T, k)
	for w := 1; w < k; w++ {
		if w == 1 {
			desplaza[w] = totales[0]
		} else {
			desplaza[w] = op(desplaza[w-1], totales[w-1])
		}
	}

	// Fase 3: cada bloque aplica su desplazamiento POR LA IZQUIERDA (importa si ⊕ no conmuta).
	for w := 1; w < k; w++ {
		wg.Go(func() {
			desde, hasta := limites(w)
			for i := desde; i < hasta; i++ {
				dst[i] = op(desplaza[w], dst[i])
			}
		})
	}
	wg.Wait()
}

// Mat es una matriz 2×2 módulo p. El producto es asociativo pero NO conmutativo.
type Mat struct{ a, b, c, d uint64 }

const p = 1_000_000_007

func MulMat(x, y Mat) Mat {
	return Mat{
		(x.a*y.a + x.b*y.c) % p, (x.a*y.b + x.b*y.d) % p,
		(x.c*y.a + x.d*y.c) % p, (x.c*y.b + x.d*y.d) % p,
	}
}

func Suma(a, b int64) int64 { return a + b }

func mejor(veces int, f func()) time.Duration {
	m := time.Duration(1 << 62)
	for range veces {
		t0 := time.Now()
		f()
		m = min(m, time.Since(t0))
	}
	return m
}

func tabla[T any](nombre string, xs []T, op func(a, b T) T) {
	dst := make([]T, len(xs)) // reservado UNA vez, fuera de la medición
	EscaneoSecuencial(xs, dst, op)
	base := mejor(5, func() { EscaneoSecuencial(xs, dst, op) })
	fmt.Printf("%-27s n = %-9d secuencial %-9v", nombre, len(xs), base.Round(10*time.Microsecond))
	for _, k := range []int{2, 4, 8} {
		d := mejor(5, func() { EscaneoParalelo(xs, dst, k, op) })
		fmt.Printf(" | k=%d %-9v (%.1f×)", k, d.Round(10*time.Microsecond), float64(base)/float64(d))
	}
	fmt.Println()
}

func main() {
	fmt.Printf("GOMAXPROCS=%d. Escaneo (suma de prefijos) con dos operaciones asociativas (mejor de 5).\n\n", runtime.GOMAXPROCS(0))
	for _, n := range []int{1_000_000, 8_000_000} {
		ints := make([]int64, n)
		for i := range ints {
			ints[i] = int64(i%9) - 3
		}
		tabla("suma de int64 (barata)", ints, Suma)

		mats := make([]Mat, n)
		for i := range mats {
			mats[i] = Mat{1, 1, 1, 0} // la matriz de Fibonacci: el prefijo i es M^(i+1)
		}
		tabla("producto 2×2 mod p (cara)", mats, MulMat)
		fmt.Println()
	}
}
