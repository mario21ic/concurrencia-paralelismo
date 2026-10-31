// Ejercicio 8: del código al runtime — qué funciones de runtime invoca el compilador por cada construcción.
// Compila un fragmento con `go build -gcflags=-S` y extrae las llamadas a runtime.*.
//
//	go run .
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const fuente = `package ejemplo

import "sync"

var mu sync.Mutex

//go:noinline
func Lanzar(f func())                  { go f() }

//go:noinline
func Enviar(c chan int, v int)         { c <- v }

//go:noinline
func Recibir(c chan int) int           { return <-c }

//go:noinline
func Cerrar(c chan int)                { close(c) }

//go:noinline
func Crear(n int) chan int             { return make(chan int, n) }

//go:noinline
func Select2(a, b chan int) int {
	select {
	case v := <-a:
		return v
	case v := <-b:
		return v
	}
}

//go:noinline
func SelectDefault(a chan int) int {
	select {
	case v := <-a:
		return v
	default:
		return -1
	}
}

//go:noinline
func Diferir(c chan int)               { defer close(c) }

//go:noinline
func Cerrojo()                         { mu.Lock(); mu.Unlock() }

//go:noinline
func Rango(c chan int) (s int)         { for v := range c { s += v }; return }

//go:noinline
func Interfaz(x any) string            { return x.(fmt.Stringer).String() }
`

var reLlamada = regexp.MustCompile(`CALL\s+(runtime\.[A-Za-z0-9_.]+|(?:internal/)?sync\.[A-Za-z0-9_.()*]+)`)
var reFunc = regexp.MustCompile(`^(ejemplo\.[A-Za-z0-9]+) STEXT`)

func main() {
	dir, err := os.MkdirTemp("", "runtime-ejemplo")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(dir)
	src := strings.Replace(fuente, `import "sync"`, "import (\n\t\"fmt\"\n\t\"sync\"\n)", 1)
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module ejemplo\n\ngo 1.26\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "e.go"), []byte(src), 0o644)

	cmd := exec.Command("go", "build", "-gcflags=-S -l=0", "./...")
	cmd.Dir = dir
	salida, _ := cmd.CombinedOutput() // el ensamblador va a stderr

	llamadas := map[string][]string{}
	var orden []string
	actual := ""
	for linea := range strings.SplitSeq(string(salida), "\n") {
		if m := reFunc.FindStringSubmatch(linea); m != nil {
			actual = m[1]
			orden = append(orden, actual)
			continue
		}
		if actual == "" {
			continue
		}
		if m := reLlamada.FindStringSubmatch(linea); m != nil && !strings.Contains(m[1], "morestack") {
			if !contiene(llamadas[actual], m[1]) {
				llamadas[actual] = append(llamadas[actual], m[1])
			}
		}
	}
	sort.Strings(orden)
	fmt.Println("construcción de Go → funciones del runtime que el compilador llama")
	fmt.Println()
	for _, f := range orden {
		fmt.Printf("  %-22s %s\n", strings.TrimPrefix(f, "ejemplo."), strings.Join(llamadas[f], ", "))
	}
	fmt.Println(`
Lectura:
  go f()            runtime.newproc        crea una G (reutiliza una de gFree) y la deja en runnext del P actual
  make(chan T, n)   runtime.makechan       reserva hchan (+ buffer) en un único bloque si T no tiene punteros
  c <- v            runtime.chansend1      cola de receptores → buffer → bloquear (gopark) en sendq
  <-c               runtime.chanrecv1      cola de emisores → buffer → bloquear en recvq
  close(c)          runtime.closechan      despierta a TODOS los bloqueados
  select (2 casos)  runtime.selectgo       ordena por dirección, bloquea todos, elige al azar entre los listos
  select + default  runtime.selectnbrecv   sin selectgo: una operación no bloqueante directa
  mu.Lock()         …Mutex.lockSlow        la vía rápida (CAS) va en línea; solo si falla se llama a lockSlow → semáforo/gopark
  defer             runtime.deferreturn    con defer 'open-coded' no hay llamada; aquí queda deferreturn por el recorrido de la pila
  x.(I)             runtime.typeAssert     conversión dinámica a interfaz con caché de itabs

Siguiente paso: compara con 'GODEBUG=schedtrace=100 go run ../01-gmp-colas' y con 'go tool trace' (módulo 12).`)
}

func contiene(s []string, x string) bool {
	for _, v := range s {
		if v == x {
			return true
		}
	}
	return false
}
