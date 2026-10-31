// Ejercicio 3: atomic.Pointer, atomic.Value y configuración recargable.
//
// Idea clave: dos atómicos NO forman una operación atómica. Para cambiar
// varios campos "a la vez" se publica UN puntero a una estructura inmutable.
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// Invariante: Limite == Nivel*10. Un lector nunca debería ver otra cosa.
type Config struct {
	Nivel  int
	Limite int
}

func nuevaConfig(nivel int) *Config { return &Config{Nivel: nivel, Limite: nivel * 10} }

func ejecutar(d time.Duration, leer func() (int, int), escribir func(nivel int)) (lecturas, incoherentes int64) {
	var wg sync.WaitGroup
	var l, inc atomic.Int64
	parar := make(chan struct{})

	wg.Go(func() { // escritor: cambia la configuración continuamente
		for n := 1; ; n++ {
			select {
			case <-parar:
				return
			default:
				escribir(n)
			}
		}
	})
	for range 4 { // lectores
		wg.Go(func() {
			for {
				select {
				case <-parar:
					return
				default:
					nivel, limite := leer()
					l.Add(1)
					if limite != nivel*10 && nivel != 0 {
						inc.Add(1)
					}
				}
			}
		})
	}
	time.Sleep(d)
	close(parar)
	wg.Wait()
	return l.Load(), inc.Load()
}

func main() {
	// A) Dos atómicos separados: el lector puede ver (nivel nuevo, limite viejo).
	var nivel, limite atomic.Int64
	l, inc := ejecutar(200*time.Millisecond,
		func() (int, int) { return int(nivel.Load()), int(limite.Load()) },
		func(n int) { nivel.Store(int64(n)); limite.Store(int64(n * 10)) },
	)
	fmt.Printf("dos atomic.Int64:  %9d lecturas, %6d INCOHERENTES\n", l, inc)

	// B) Un atomic.Pointer a una Config inmutable: se publica entera.
	var cfg atomic.Pointer[Config]
	cfg.Store(nuevaConfig(1))
	l, inc = ejecutar(200*time.Millisecond,
		func() (int, int) { c := cfg.Load(); return c.Nivel, c.Limite },
		func(n int) { cfg.Store(nuevaConfig(n)) }, // NUNCA mutar *cfg.Load() in situ
	)
	fmt.Printf("atomic.Pointer:    %9d lecturas, %6d incoherentes\n", l, inc)

	// C) Actualización condicional con CompareAndSwap sobre el puntero.
	viejo := cfg.Load()
	nuevo := nuevaConfig(viejo.Nivel + 1)
	fmt.Println("\nCAS con el puntero que leí:   ", cfg.CompareAndSwap(viejo, nuevo))
	fmt.Println("CAS con un puntero ya obsoleto:", cfg.CompareAndSwap(viejo, nuevaConfig(99)))

	// D) atomic.Value: anterior a los genéricos; guarda `any`.
	var v atomic.Value
	fmt.Println("\nValue.Load() sin Store:", v.Load())
	v.Store("texto")
	fmt.Println("Value.Load():", v.Load())

	intentar := func(nombre string, f func()) {
		defer func() { fmt.Printf("%-28s -> panic: %v\n", nombre, recover()) }()
		f()
	}
	intentar("Store de otro tipo:", func() { v.Store(42) })
	intentar("Store(nil):", func() { v.Store(nil) })
}
