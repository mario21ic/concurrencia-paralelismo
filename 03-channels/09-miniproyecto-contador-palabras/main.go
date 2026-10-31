// Mini-proyecto 1: contador de palabras concurrente sobre varios archivos.
//
// Diseño (fan-out / fan-in con channels):
//
//	archivos ──► [jobs] ──► N workers ──► [resultados] ──► agregador (main)
//
// Cada worker cuenta las palabras de un archivo en un map LOCAL (sin locks)
// y envía ese map por un channel. Solo el agregador toca el map global,
// por lo que no hay memoria compartida entre goroutines.
//
//	go run . -dir textos -top 10 -workers 4
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"unicode"
)

type resultado struct {
	archivo  string
	palabras map[string]int
	err      error
}

// contarArchivo cuenta palabras (letras Unicode, en minúscula, longitud >= minLen).
func contarArchivo(ruta string, minLen int) (map[string]int, error) {
	f, err := os.Open(ruta)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cuenta := make(map[string]int)
	sc := bufio.NewScanner(f)
	sc.Split(bufio.ScanWords)
	for sc.Scan() {
		p := strings.ToLower(strings.TrimFunc(sc.Text(), func(r rune) bool { return !unicode.IsLetter(r) }))
		if len([]rune(p)) >= minLen {
			cuenta[p]++
		}
	}
	return cuenta, sc.Err()
}

func listar(dir string) ([]string, error) {
	return filepath.Glob(filepath.Join(dir, "*.txt"))
}

// contarSecuencial sirve de referencia para verificar la versión concurrente.
func contarSecuencial(archivos []string, minLen int) (map[string]int, error) {
	total := make(map[string]int)
	for _, a := range archivos {
		m, err := contarArchivo(a, minLen)
		if err != nil {
			return nil, err
		}
		for p, n := range m {
			total[p] += n
		}
	}
	return total, nil
}

// contarConcurrente reparte los archivos entre `workers` goroutines.
func contarConcurrente(archivos []string, workers, minLen int) (map[string]int, []error) {
	jobs := make(chan string)
	resultados := make(chan resultado)

	// productor de trabajos
	go func() {
		defer close(jobs)
		for _, a := range archivos {
			jobs <- a
		}
	}()

	// workers (fan-out)
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for a := range jobs {
				m, err := contarArchivo(a, minLen)
				resultados <- resultado{archivo: a, palabras: m, err: err}
			}
		})
	}

	// cierra resultados cuando todos los workers terminan
	go func() {
		wg.Wait()
		close(resultados)
	}()

	// agregador (fan-in): único dueño del map global
	total := make(map[string]int)
	var errs []error
	for r := range resultados {
		if r.err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", r.archivo, r.err))
			continue
		}
		for p, n := range r.palabras {
			total[p] += n
		}
	}
	return total, errs
}

type par struct {
	palabra string
	n       int
}

func top(m map[string]int, k int) []par {
	pares := make([]par, 0, len(m))
	for p, n := range m {
		pares = append(pares, par{p, n})
	}
	slices.SortFunc(pares, func(a, b par) int {
		if a.n != b.n {
			return b.n - a.n // más frecuentes primero
		}
		return strings.Compare(a.palabra, b.palabra) // desempate estable
	})
	if k < len(pares) {
		pares = pares[:k]
	}
	return pares
}

func main() {
	dir := flag.String("dir", "textos", "directorio con archivos .txt")
	k := flag.Int("top", 10, "cuántas palabras mostrar")
	workers := flag.Int("workers", 4, "número de workers")
	minLen := flag.Int("min", 4, "longitud mínima de palabra")
	flag.Parse()

	archivos, err := listar(*dir)
	if err != nil || len(archivos) == 0 {
		fmt.Fprintln(os.Stderr, "sin archivos .txt en", *dir)
		os.Exit(1)
	}

	total, errs := contarConcurrente(archivos, *workers, *minLen)
	for _, e := range errs {
		fmt.Fprintln(os.Stderr, "error:", e)
	}

	fmt.Printf("%d archivos, %d palabras distintas\n", len(archivos), len(total))
	for i, p := range top(total, *k) {
		fmt.Printf("%2d. %-16s %d\n", i+1, p.palabra, p.n)
	}
}
