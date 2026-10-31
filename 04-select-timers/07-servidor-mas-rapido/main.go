// Ejercicio 7: consultar 3 servidores simulados y quedarse con el más rápido.
//
//	go run .
//
// Los servidores tienen latencias y fallos aleatorios. Se devuelve la primera
// respuesta EXITOSA; si ninguna llega antes del timeout, se devuelve error.
package main

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"runtime"
	"time"
)

type respuesta struct {
	servidor string
	dato     string
	demora   time.Duration
	err      error
}

// consultar simula un servidor con latencia aleatoria y probabilidad de fallo.
func consultar(nombre string, base time.Duration, pFallo float64) respuesta {
	ini := time.Now()
	time.Sleep(base + time.Duration(rand.IntN(100))*time.Millisecond)
	r := respuesta{servidor: nombre, demora: time.Since(ini)}
	if rand.Float64() < pFallo {
		r.err = errors.New("fallo del servidor")
		return r
	}
	r.dato = "datos de " + nombre
	return r
}

type servidor struct {
	nombre string
	base   time.Duration
	pFallo float64
}

// primero lanza una goroutine por servidor y devuelve la primera respuesta
// correcta. El channel de resultados tiene capacidad len(servidores): así los
// servidores perdedores pueden enviar su respuesta y terminar, sin fugarse.
func primero(servidores []servidor, timeout time.Duration) (respuesta, error) {
	res := make(chan respuesta, len(servidores))
	for _, s := range servidores {
		go func() { res <- consultar(s.nombre, s.base, s.pFallo) }()
	}

	plazo := time.NewTimer(timeout)
	defer plazo.Stop()

	var errs []error
	for range servidores {
		select {
		case r := <-res:
			if r.err == nil {
				return r, nil
			}
			errs = append(errs, fmt.Errorf("%s: %w", r.servidor, r.err))
		case <-plazo.C:
			if len(errs) > 0 {
				return respuesta{}, fmt.Errorf("timeout de %v (errores previos: %w)", timeout, errors.Join(errs...))
			}
			return respuesta{}, fmt.Errorf("timeout de %v", timeout)
		}
	}
	return respuesta{}, fmt.Errorf("todos fallaron: %w", errors.Join(errs...))
}

// primeroConFuga: igual, pero con channel SIN buffer. Los perdedores quedan
// bloqueados para siempre en `res <- ...`.
func primeroConFuga(servidores []servidor) respuesta {
	res := make(chan respuesta)
	for _, s := range servidores {
		go func() { res <- consultar(s.nombre, s.base, s.pFallo) }()
	}
	return <-res
}

func main() {
	servidores := []servidor{
		{"eu-oeste", 80 * time.Millisecond, 0.2},
		{"us-este", 40 * time.Millisecond, 0.3},
		{"asia", 120 * time.Millisecond, 0.1},
	}

	for i := 1; i <= 5; i++ {
		ini := time.Now()
		r, err := primero(servidores, 300*time.Millisecond)
		if err != nil {
			fmt.Printf("#%d error: %v\n", i, err)
			continue
		}
		fmt.Printf("#%d ganó %-9s en %v -> %s\n", i, r.servidor, time.Since(ini).Round(time.Millisecond), r.dato)
	}

	// Timeout
	_, err := primero([]servidor{{"lento", 2 * time.Second, 0}}, 100*time.Millisecond)
	fmt.Println("timeout:", err)

	// Comparar fugas.
	time.Sleep(2500 * time.Millisecond) // deja terminar al "lento" del paso anterior
	base := runtime.NumGoroutine()
	for range 10 {
		primero(servidores, 300*time.Millisecond)
	}
	time.Sleep(500 * time.Millisecond)
	fmt.Println("\ngoroutines fugadas con buffer:", runtime.NumGoroutine()-base)

	for range 10 {
		primeroConFuga(servidores)
	}
	time.Sleep(500 * time.Millisecond)
	fmt.Println("goroutines fugadas sin buffer:", runtime.NumGoroutine()-base, "(2 por llamada)")
}
