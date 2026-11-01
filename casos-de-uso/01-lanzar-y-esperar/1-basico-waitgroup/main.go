// Caso 1.1 (básico): hacer varias esperas a la vez y esperar a que TODAS terminen.
//
// Problema: una página necesita datos de 8 sitios. Pedirlos uno tras otro suma las esperas; pedirlos
// a la vez las solapa. Herramienta: `go` + sync.WaitGroup.Go.
//
//	go run .
package main

import (
	"fmt"
	"sync"
	"time"
)

const latencia = 100 * time.Millisecond // lo que tarda cada "petición" (simulada)

func descargar(url string) string {
	time.Sleep(latencia) // aquí iría http.Get: la goroutine se aparca y no consume CPU
	return "contenido de " + url
}

// Secuencial: el tiempo total es la SUMA de las esperas.
func Secuencial(urls []string) []string {
	res := make([]string, len(urls))
	for i, u := range urls {
		res[i] = descargar(u)
	}
	return res
}

// Paralelo: una goroutine por URL; el tiempo total es la espera MÁS LARGA.
//
// Dos detalles que evitan los errores clásicos:
//   - cada goroutine escribe SOLO en su posición res[i]: no hay dato compartido que proteger;
//   - wg.Wait() garantiza que todas terminaron antes de leer res.
func Paralelo(urls []string) []string {
	res := make([]string, len(urls))
	var wg sync.WaitGroup
	for i, u := range urls { // Go 1.22+: i y u son nuevos en cada vuelta (se pueden capturar sin copiar)
		wg.Go(func() { res[i] = descargar(u) })
	}
	wg.Wait()
	return res
}

func main() {
	urls := []string{"a.com", "b.com", "c.com", "d.com", "e.com", "f.com", "g.com", "h.com"}

	t0 := time.Now()
	Secuencial(urls)
	fmt.Printf("secuencial: %v\n", time.Since(t0).Round(10*time.Millisecond))

	t0 = time.Now()
	res := Paralelo(urls)
	fmt.Printf("paralelo:   %v   (primer resultado: %q)\n", time.Since(t0).Round(10*time.Millisecond), res[0])
}
