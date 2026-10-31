package almacen

import (
	"fmt"
	"math/rand/v2"
	"os"
	"sync"
	"sync/atomic"
	"testing"
)

func implementaciones() map[string]func() Almacen {
	return map[string]func() Almacen{
		"rw":    func() Almacen { return NuevoRW() },
		"shard": func() Almacen { return NuevoFragmentado() },
	}
}

func TestLecturaEscritura(t *testing.T) {
	for nombre, nuevo := range implementaciones() {
		a := nuevo()
		if _, ok := a.Leer(1); ok {
			t.Errorf("%s: clave ausente no debe existir", nombre)
		}
		a.Escribir(1, 42)
		if v, ok := a.Leer(1); !ok || v != 42 {
			t.Errorf("%s: Leer(1) = %d,%v", nombre, v, ok)
		}
	}
}

func TestConcurrente(t *testing.T) {
	for nombre, nuevo := range implementaciones() {
		a := nuevo()
		var wg sync.WaitGroup
		for g := range 8 {
			wg.Go(func() {
				for i := range 5000 {
					k := uint64(g*5000 + i)
					a.Escribir(k, k*2)
				}
			})
		}
		wg.Wait()
		for k := range uint64(40000) {
			if v, ok := a.Leer(k); !ok || v != k*2 {
				t.Fatalf("%s: Leer(%d) = %d,%v", nombre, k, v, ok)
			}
		}
	}
}

// ---------------------------------------------------------------- benchmarks

// implementación a medir: IMPL=rw (por defecto) o IMPL=shard. Así el MISMO benchmark, con el
// MISMO nombre, produce los datos "antes" y "después" que compara benchstat.
func implElegida() Almacen {
	switch os.Getenv("IMPL") {
	case "shard":
		return NuevoFragmentado()
	default:
		return NuevoRW()
	}
}

const claves = 4096

func BenchmarkAlmacen(b *testing.B) {
	for _, pctLectura := range []int{50, 95, 100} {
		b.Run(fmt.Sprintf("lecturas=%d%%", pctLectura), func(b *testing.B) {
			a := implElegida()
			for k := range uint64(claves) { // preparación: FUERA de la medición
				a.Escribir(k, k)
			}
			var semilla atomic.Uint64
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				r := rand.New(rand.NewPCG(semilla.Add(1), 7)) // un generador por goroutine: sin contención
				for pb.Next() {
					k := r.Uint64N(claves)
					if r.IntN(100) < pctLectura {
						a.Leer(k)
					} else {
						a.Escribir(k, k)
					}
				}
			})
		})
	}
}

// ---- Trampas clásicas de los microbenchmarks ----

func cuadrado(x int) int { return x * x }

// Estilo antiguo: la función es pura, se puede inlinar y su resultado no se usa, así que el
// compilador puede eliminar la llamada por completo y el benchmark mide el bucle vacío.
func BenchmarkTrampaViejoEstilo(b *testing.B) {
	for i := 0; i < b.N; i++ {
		cuadrado(7)
	}
}

// Solución clásica: acumular el resultado en una variable global ("sumidero").
var sumidero int

func BenchmarkConSumidero(b *testing.B) {
	r := 0
	for i := 0; i < b.N; i++ {
		r = cuadrado(i)
	}
	sumidero = r
}

// b.Loop (Go 1.24+): mantiene viva la llamada y sus argumentos/resultados, y excluye de la
// medición todo lo que hay antes del bucle. Es la forma recomendada.
func BenchmarkConLoop(b *testing.B) {
	for b.Loop() {
		cuadrado(7)
	}
}

func suma(n int) int {
	s := 0
	for i := range n {
		s += i
	}
	return s
}

// Métricas propias: además de ns/op se puede publicar cualquier magnitud.
func BenchmarkMetricaPropia(b *testing.B) {
	var ops int
	for b.Loop() {
		ops += suma(1000)
	}
	b.ReportMetric(float64(ops)/float64(b.N), "suma/op")
}
