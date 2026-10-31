package main

import (
	"bufio"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var (
	reGrupo   = regexp.MustCompile(`^(\d+) @ `)
	reEstado  = regexp.MustCompile(`^goroutine \d+ \[([^\],]+)`)
	reFuncion = regexp.MustCompile(`^#\s+0x[0-9a-f]+\s+(\S+?)\+0x[0-9a-f]+\s+(\S+)`)
)

// Grupo es un conjunto de goroutines con la misma pila (formato debug=1).
type Grupo struct {
	N       int
	Funcion string // primera función de la pila (donde está parada)
	Origen  string // archivo:línea
}

// AgruparPilas interpreta la salida de pprof "goroutine" con debug=1 y devuelve
// los grupos ordenados por tamaño. El grupo mayor suele ser el culpable de una fuga.
func AgruparPilas(debug1 string) []Grupo {
	var grupos []Grupo
	var actual *Grupo
	sc := bufio.NewScanner(strings.NewReader(debug1))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		linea := sc.Text()
		if m := reGrupo.FindStringSubmatch(linea); m != nil {
			n, _ := strconv.Atoi(m[1])
			grupos = append(grupos, Grupo{N: n})
			actual = &grupos[len(grupos)-1]
			continue
		}
		if actual != nil && actual.Funcion == "" {
			if m := reFuncion.FindStringSubmatch(linea); m != nil {
				actual.Funcion, actual.Origen = m[1], m[2]
			}
		}
	}
	slices.SortFunc(grupos, func(a, b Grupo) int { return b.N - a.N })
	return grupos
}

// ContarPorEstado interpreta la salida con debug=2 (una cabecera por goroutine,
// p. ej. "goroutine 7 [chan receive, 2 minutes]:") y cuenta por estado.
func ContarPorEstado(debug2 string) map[string]int {
	cuenta := map[string]int{}
	sc := bufio.NewScanner(strings.NewReader(debug2))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		if m := reEstado.FindStringSubmatch(sc.Text()); m != nil {
			cuenta[m[1]]++
		}
	}
	return cuenta
}
