// Ejercicio 3: perfil de goroutines — qué están haciendo y dónde se acumulan.
//
//	go run .              # fuga activa
//	go run . -arreglado   # sin fuga
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	_ "net/http/pprof" // registra /debug/pprof/* en http.DefaultServeMux
	"runtime/pprof"
	"slices"
	"sync"
	"time"
)

// goroutinesEnEstadosDistintos deja goroutines bloqueadas por motivos diferentes.
func goroutinesEnEstadosDistintos() (liberar func()) {
	parar := make(chan struct{})
	var wg sync.WaitGroup

	recibir := make(chan int)
	enviar := make(chan int) // sin receptor
	var mu sync.Mutex
	mu.Lock() // lo retiene main hasta liberar()
	cond := sync.NewCond(&sync.Mutex{})

	for range 3 {
		wg.Go(func() { <-recibir }) // chan receive
	}
	for range 2 {
		wg.Go(func() { enviar <- 1 }) // chan send
	}
	wg.Go(func() { // select
		select {
		case <-recibir:
		case <-parar:
		}
	})
	for range 2 {
		wg.Go(func() { time.Sleep(time.Hour) }) // sleep
	}
	for range 4 {
		wg.Go(func() { mu.Lock(); mu.Unlock() }) // sync.Mutex.Lock
	}
	wg.Go(func() { // sync.Cond.Wait
		cond.L.Lock()
		cond.Wait()
		cond.L.Unlock()
	})

	// E/S de red: lectura bloqueada en un socket TCP real.
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	conn, _ := net.Dial("tcp", ln.Addr().String())
	srv, _ := ln.Accept()
	wg.Go(func() { io.ReadAll(conn) }) // IO wait

	time.Sleep(50 * time.Millisecond) // deja que todas lleguen a su punto de bloqueo

	return func() {
		mu.Unlock()
		close(recibir)
		cond.Broadcast()
		srv.Close()
		conn.Close()
		ln.Close()
		close(parar)
		// (las de Sleep y chan send quedan vivas; el programa termina de todos modos)
	}
}

func volcar(debug int) string {
	var b bytes.Buffer
	pprof.Lookup("goroutine").WriteTo(&b, debug)
	return b.String()
}

func descargar(url string) string {
	resp, err := http.Get(url)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

func main() {
	arreglado := flag.Bool("arreglado", false, "versión del handler sin fuga")
	flag.Parse()

	// ---- Parte 1: estados de las goroutines ----
	liberar := goroutinesEnEstadosDistintos()
	estados := ContarPorEstado(volcar(2))
	fmt.Println("== Parte 1: goroutines por estado (perfil con debug=2) ==")
	claves := make([]string, 0, len(estados))
	for k := range estados {
		claves = append(claves, k)
	}
	slices.SortFunc(claves, func(a, b string) int { return estados[b] - estados[a] })
	for _, k := range claves {
		fmt.Printf("  %-22s %d\n", k, estados[k])
	}
	liberar()
	time.Sleep(50 * time.Millisecond)

	// ---- Parte 2: encontrar una fuga con el endpoint /debug/pprof/goroutine ----
	http.HandleFunc("/pedido", func(w http.ResponseWriter, r *http.Request) {
		// Sin buffer: si el handler se rinde por timeout, nadie recibirá y el envío
		// bloquea para siempre. Con buffer 1 la goroutine siempre puede terminar.
		resultado := make(chan string)
		if *arreglado {
			resultado = make(chan string, 1)
		}
		go func() {
			time.Sleep(time.Millisecond) // trabajo de fondo
			resultado <- "ok"
		}()
		select {
		case v := <-resultado:
			fmt.Fprint(w, v)
		case <-time.After(100 * time.Microsecond): // plazo demasiado corto: el handler se rinde
			http.Error(w, "timeout", http.StatusGatewayTimeout)
		}
	})
	srv := httptest.NewServer(nil) // usa http.DefaultServeMux (incluye pprof)
	defer srv.Close()

	antes := total(descargar(srv.URL + "/debug/pprof/goroutine?debug=1"))
	for range 200 {
		descargar(srv.URL + "/pedido")
	}
	time.Sleep(100 * time.Millisecond)
	salida := descargar(srv.URL + "/debug/pprof/goroutine?debug=1")
	despues := total(salida)

	fmt.Printf("\n== Parte 2: tras 200 peticiones (arreglado=%v) ==\n", *arreglado)
	fmt.Printf("  goroutines: antes=%d después=%d (+%d)\n", antes, despues, despues-antes)
	fmt.Println("  grupos de pilas más grandes:")
	for i, g := range AgruparPilas(salida) {
		if i >= 3 {
			break
		}
		fmt.Printf("    %4d × %s (%s)\n", g.N, g.Funcion, g.Origen)
	}
}

// total lee la primera línea "goroutine profile: total N".
func total(debug1 string) int {
	var n int
	fmt.Sscanf(debug1, "goroutine profile: total %d", &n)
	return n
}
