// Ejercicio 2: llamadas al sistema bloqueantes frente al netpoller.
//
//	go run . -modo=comparar            # hilos del SO consumidos por cada tipo de espera
//	go run . -modo=progreso            # con GOMAXPROCS=1, otra goroutine sigue avanzando (handoff del P)
//	go run . -modo=limite              # el límite de hilos (SetMaxThreads) y su fallo fatal
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"
	"runtime/metrics"
	"sync"
	"syscall"
	"time"
)

func hilos() uint64 {
	s := []metrics.Sample{{Name: "/sched/threads/total:threads"}}
	metrics.Read(s)
	return s[0].Value.Uint64()
}

// caso deja `n` goroutines esperando de un modo concreto y devuelve los hilos del SO
// antes y después. Se ejecuta en un proceso aparte para que los resultados no se mezclen
// (el runtime conserva los hilos creados, aunque queden ociosos).
func caso(tipo string, n int) (antes, despues uint64) {
	antes = hilos()
	var wg sync.WaitGroup
	var cierres []func()

	switch tipo {
	case "canal": // goroutine aparcada en un channel: no ocupa ningún hilo
		c := make(chan struct{})
		for range n {
			wg.Go(func() { <-c })
		}
		cierres = append(cierres, func() { close(c) })

	case "sleep": // goroutine aparcada en un timer: tampoco
		for range n {
			wg.Go(func() { time.Sleep(time.Second) })
		}

	case "poller": // lectura de un pipe de os.Pipe: no bloqueante + netpoller (estado "IO wait")
		for range n {
			r, w, _ := os.Pipe()
			wg.Go(func() { r.Read(make([]byte, 1)) })
			cierres = append(cierres, func() { w.Close(); r.Close() })
		}

	case "syscall": // read(2) BLOQUEANTE sobre un pipe sin registrar en el poller: ocupa un hilo cada una
		for range n {
			var p [2]int
			syscall.Pipe(p[:])
			wg.Go(func() { syscall.Read(p[0], make([]byte, 1)) })
			cierres = append(cierres, func() { syscall.Write(p[1], []byte{1}); syscall.Close(p[0]); syscall.Close(p[1]) })
		}
	}

	time.Sleep(300 * time.Millisecond) // deja que todas lleguen a su espera
	despues = hilos()
	for _, c := range cierres {
		c()
	}
	wg.Wait()
	return
}

func comparar() {
	fmt.Printf("%-34s %6s %12s %12s\n", "tipo de espera", "goroutines", "hilos antes", "hilos después")
	for _, c := range []struct{ tipo, desc string }{
		{"canal", "bloqueadas en un channel"},
		{"sleep", "en time.Sleep"},
		{"poller", "leyendo un os.Pipe (netpoller)"},
		{"syscall", "en read(2) bloqueante (syscall)"},
	} {
		for _, n := range []int{100, 1000} {
			out, err := exec.Command(os.Args[0], "-modo=caso", "-tipo="+c.tipo, fmt.Sprint("-n=", n)).Output()
			if err != nil {
				fmt.Printf("%-34s %6d  error: %v\n", c.desc, n, err)
				continue
			}
			var a, d uint64
			fmt.Sscanf(string(bytes.TrimSpace(out)), "%d %d", &a, &d)
			fmt.Printf("%-34s %6d %12d %12d\n", c.desc, n, a, d)
		}
	}
}

// progreso: con UN solo P, una goroutine queda bloqueada en una syscall bloqueante y otra
// sigue contando. Si el P no se cediera, la segunda no avanzaría.
func progreso() {
	runtime.GOMAXPROCS(1)
	var p [2]int
	syscall.Pipe(p[:])

	var cuenta, congelada int64
	parar := make(chan struct{})
	go func() {
		for {
			select {
			case <-parar:
				return
			default:
				cuenta++
				time.Sleep(100 * time.Microsecond)
			}
		}
	}()
	time.Sleep(50 * time.Millisecond)
	antes := cuenta

	listo := make(chan struct{})
	go func() {
		close(listo)
		syscall.Read(p[0], make([]byte, 1)) // se queda en el kernel
	}()
	<-listo
	time.Sleep(300 * time.Millisecond)
	congelada = cuenta - antes

	fmt.Printf("GOMAXPROCS=1: una goroutine lleva 300 ms bloqueada en read(2)\n")
	fmt.Printf("  la otra goroutine avanzó %d pasos en esos 300 ms (50 ms previos: %d pasos)\n", congelada, antes)
	fmt.Printf("  el ritmo no cambió: el único P se cedió a otro hilo mientras el primero estaba en el kernel (handoff)\n")
	syscall.Write(p[1], []byte{1})
	close(parar)
}

func limite() {
	debug.SetMaxThreads(40)
	fmt.Println("SetMaxThreads(40) y 100 goroutines en read(2) bloqueante...")
	for range 100 {
		var p [2]int
		syscall.Pipe(p[:])
		go syscall.Read(p[0], make([]byte, 1))
	}
	time.Sleep(time.Second)
	fmt.Println("no se llega aquí")
}

func main() {
	modo := flag.String("modo", "comparar", "comparar|progreso|limite|caso")
	tipo := flag.String("tipo", "canal", "(interno)")
	n := flag.Int("n", 100, "(interno)")
	flag.Parse()
	switch *modo {
	case "comparar":
		comparar()
	case "caso":
		a, d := caso(*tipo, *n)
		fmt.Println(a, d)
	case "progreso":
		progreso()
	case "limite":
		limite()
	}
}
