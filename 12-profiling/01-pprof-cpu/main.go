// Ejercicio 1: perfil de CPU con runtime/pprof.
//
//	go run . -modo=lento  -perfil lento.pprof
//	go run . -modo=rapido -perfil rapido.pprof
//	go tool pprof -top -nodecount=12 lento.pprof
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"sync"
	"time"
)

// claveLenta construye la clave con varias asignaciones intermedias.
func claveLenta(i int) string {
	s := fmt.Sprintf("usuario-%d-%s", i, "region")
	s = strings.ToUpper(s)
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

// claveRapida produce EXACTAMENTE el mismo resultado reutilizando búferes.
type claveRapida struct {
	buf [64]byte
	dst [64]byte
}

func (c *claveRapida) calcular(i int) []byte {
	b := append(c.buf[:0], "USUARIO-"...)
	b = strconv.AppendInt(b, int64(i), 10)
	b = append(b, "-REGION"...)
	h := sha256.Sum256(b)
	hex.Encode(c.dst[:], h[:])
	return c.dst[:]
}

// trabajar reparte n elementos entre `workers` goroutines y devuelve una suma de
// comprobación (para verificar que ambas versiones calculan lo mismo y que el
// compilador no elimine el trabajo).
func trabajar(modo string, n, workers int) uint64 {
	var wg sync.WaitGroup
	sumas := make([]uint64, workers)
	for w := range workers {
		wg.Go(func() {
			var c claveRapida
			var s uint64
			for i := w; i < n; i += workers {
				if modo == "lento" {
					k := claveLenta(i)
					s += uint64(k[0]) + uint64(k[len(k)-1])
				} else {
					k := c.calcular(i)
					s += uint64(k[0]) + uint64(k[len(k)-1])
				}
			}
			sumas[w] = s
		})
	}
	wg.Wait()
	var total uint64
	for _, s := range sumas {
		total += s
	}
	return total
}

func main() {
	modo := flag.String("modo", "lento", "lento|rapido")
	n := flag.Int("n", 5_000_000, "elementos")
	perfil := flag.String("perfil", "", "archivo de salida del perfil de CPU")
	flag.Parse()

	if *perfil != "" {
		f, err := os.Create(*perfil)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		if err := pprof.StartCPUProfile(f); err != nil { // muestrea ~100 veces por segundo
			panic(err)
		}
		defer pprof.StopCPUProfile()
	}

	workers := runtime.NumCPU()
	ini := time.Now()
	suma := trabajar(*modo, *n, workers)
	fmt.Printf("modo=%s n=%d workers=%d suma=%d tiempo=%v\n", *modo, *n, workers, suma, time.Since(ini).Round(time.Millisecond))
}
