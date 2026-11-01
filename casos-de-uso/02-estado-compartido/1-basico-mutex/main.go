// Caso 2.1 (básico): dos goroutines modifican la misma variable → hay que proteger el dato.
//
// Problema: un contador de visitas atendido por 1 000 goroutines. `visitas++` NO es atómico (son tres
// pasos: leer, sumar, escribir) y las actualizaciones se pisan.
//
//	go run .            # las dos versiones correctas
//	go run -race . -mal # la versión con carrera: el detector la señala (y el resultado sale mal)
package main

import (
	"flag"
	"fmt"
	"sync"
	"sync/atomic"
)

const (
	goroutines = 1000
	cadaUna    = 1000
)

// ContarMal: carrera de datos. Resultado impredecible (casi siempre menor de lo esperado).
func ContarMal() int {
	var visitas int
	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			for range cadaUna {
				visitas++ // ← leer-sumar-escribir sin protección
			}
		})
	}
	wg.Wait()
	return visitas
}

// ContarMutex: un cerrojo hace que la sección crítica la ejecute una goroutine cada vez.
// Úsalo cuando el dato es una estructura (varios campos que deben cambiar juntos) o la operación
// es algo más que sumar.
func ContarMutex() int {
	var (
		mu      sync.Mutex
		visitas int
		wg      sync.WaitGroup
	)
	for range goroutines {
		wg.Go(func() {
			for range cadaUna {
				mu.Lock()
				visitas++
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	return visitas
}

// ContarAtomic: una única instrucción atómica de CPU. Úsalo cuando el estado es UN solo número o
// puntero y la operación es simple (sumar, comparar-e-intercambiar). Más barato que el mutex.
func ContarAtomic() int {
	var visitas atomic.Int64
	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			for range cadaUna {
				visitas.Add(1)
			}
		})
	}
	wg.Wait()
	return int(visitas.Load())
}

func main() {
	mal := flag.Bool("mal", false, "ejecutar también la versión con carrera de datos")
	flag.Parse()
	esperado := goroutines * cadaUna
	fmt.Printf("esperado:   %d\n", esperado)
	fmt.Printf("con mutex:  %d\n", ContarMutex())
	fmt.Printf("con atomic: %d\n", ContarAtomic())
	if *mal {
		got := ContarMal()
		fmt.Printf("sin proteger: %d  (se pierden %d actualizaciones)\n", got, esperado-got)
	}
}
