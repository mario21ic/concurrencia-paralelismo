// Ejercicio 8: diagnóstico integrador.
//
// Un servicio con TRES problemas de rendimiento/concurrencia ocultos (uno por
// endpoint) y un generador de carga. El objetivo es encontrarlos con pprof.
//
//	go run . -modo=servidor -addr 127.0.0.1:18090                 # con los 3 problemas
//	go run . -modo=servidor -addr 127.0.0.1:18090 -arreglado      # corregido
//	go run . -modo=carga -url http://127.0.0.1:18090 -dur 5s      # mide latencias y throughput
//
//	go tool pprof -top 'http://127.0.0.1:18090/debug/pprof/profile?seconds=3'   # CPU
//	go tool pprof -top  http://127.0.0.1:18090/debug/pprof/mutex                # contención
//	go tool pprof -top  http://127.0.0.1:18090/debug/pprof/goroutine            # fugas
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	modo := flag.String("modo", "servidor", "servidor|carga")
	addr := flag.String("addr", "127.0.0.1:18090", "dirección del servidor")
	arreglado := flag.Bool("arreglado", false, "versión corregida")
	url := flag.String("url", "http://127.0.0.1:18090", "URL base para la carga")
	dur := flag.Duration("dur", 5*time.Second, "duración de la carga")
	flag.Parse()

	switch *modo {
	case "servidor":
		servidor(*addr, *arreglado)
	case "carga":
		carga(*url, *dur)
	default:
		flag.Usage()
	}
}

// ---------------------------------------------------------------- servidor

var (
	mu       sync.Mutex
	contador int64
	atomico  atomic.Int64
)

func busy(d time.Duration) {
	fin := time.Now().Add(d)
	for time.Now().Before(fin) {
	}
}

func servidor(addr string, arreglado bool) {
	runtime.SetMutexProfileFraction(5) // 1 de cada 5 eventos de contención

	// PROBLEMA 1 (/cpu): construcción de la clave con muchas asignaciones.
	http.HandleFunc("/cpu", func(w http.ResponseWriter, r *http.Request) {
		i := int(time.Now().UnixNano() % 1000)
		var k string
		if arreglado {
			b := make([]byte, 0, 32)
			b = append(b, "USUARIO-"...)
			b = strconv.AppendInt(b, int64(i), 10)
			b = append(b, "-REGION"...)
			h := sha256.Sum256(b)
			k = hex.EncodeToString(h[:4])
		} else {
			for range 2000 { // "validaciones" repetidas (trabajo redundante)
				s := strings.ToUpper(fmt.Sprintf("usuario-%d-%s", i, "region"))
				h := sha256.Sum256([]byte(s))
				k = hex.EncodeToString(h[:4])
			}
		}
		fmt.Fprint(w, k)
	})

	// PROBLEMA 2 (/mutex): trabajo costoso con el lock global tomado.
	http.HandleFunc("/mutex", func(w http.ResponseWriter, r *http.Request) {
		if arreglado {
			busy(500 * time.Microsecond) // el trabajo va FUERA de la sección crítica
			atomico.Add(1)
		} else {
			mu.Lock()
			busy(500 * time.Microsecond)
			contador++
			mu.Unlock()
		}
		fmt.Fprint(w, "ok")
	})

	// PROBLEMA 3 (/fuga): una goroutine de "auditoría" que se queda bloqueada.
	http.HandleFunc("/fuga", func(w http.ResponseWriter, r *http.Request) {
		auditoria := make(chan string) // sin buffer: si nadie recibe, el envío bloquea
		if arreglado {
			auditoria = make(chan string, 1)
		}
		go func() {
			time.Sleep(2 * time.Millisecond) // la auditoría tarda más que el plazo del handler
			auditoria <- "registrado"
		}()
		select {
		case v := <-auditoria:
			fmt.Fprint(w, v)
		case <-time.After(500 * time.Microsecond):
			fmt.Fprint(w, "ok (sin auditoría)")
		}
	})

	log.Printf("servidor en %s (arreglado=%v, pid con GOMAXPROCS=%d)", addr, arreglado, runtime.GOMAXPROCS(0))
	log.Fatal(http.ListenAndServe(addr, nil))
}

// ------------------------------------------------------------------- carga

type medida struct {
	n    atomic.Int64
	mu   sync.Mutex
	lats []time.Duration
}

func carga(base string, dur time.Duration) {
	endpoints := []string{"/cpu", "/mutex", "/fuga"}
	medidas := map[string]*medida{}
	var wg sync.WaitGroup
	fin := time.Now().Add(dur)
	cliente := &http.Client{Transport: &http.Transport{MaxIdleConnsPerHost: 64}}

	for _, ep := range endpoints {
		m := &medida{}
		medidas[ep] = m
		for range 6 { // 6 clientes por endpoint
			wg.Go(func() {
				var locales []time.Duration
				for time.Now().Before(fin) {
					ini := time.Now()
					resp, err := cliente.Get(base + ep)
					if err != nil {
						continue
					}
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
					locales = append(locales, time.Since(ini))
				}
				m.n.Add(int64(len(locales)))
				m.mu.Lock()
				m.lats = append(m.lats, locales...)
				m.mu.Unlock()
			})
		}
	}
	wg.Wait()

	fmt.Printf("%-8s %10s %10s %10s %10s\n", "endpoint", "peticiones", "req/s", "p50", "p99")
	for _, ep := range endpoints {
		m := medidas[ep]
		slices.Sort(m.lats)
		q := func(p float64) time.Duration {
			if len(m.lats) == 0 {
				return 0
			}
			return m.lats[int(float64(len(m.lats)-1)*p)].Round(10 * time.Microsecond)
		}
		fmt.Printf("%-8s %10d %10.0f %10v %10v\n", ep, m.n.Load(), float64(m.n.Load())/dur.Seconds(), q(0.50), q(0.99))
	}
}
