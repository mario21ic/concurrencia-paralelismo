// Ejercicio 10: servidor HTTP bajo sobrecarga — sin límite, con cancelación, con descarte de carga y con cola acotada.
//
//	go run .
package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"time"
)

type resultado struct {
	ok, rechazados, caducados int
	lat                       []time.Duration
	trabajoInutil             int64 // operaciones del backend terminadas que NADIE recibió
	picoGoroutines            int
	rps                       float64
}

func p(d []time.Duration, q float64) time.Duration {
	if len(d) == 0 {
		return 0
	}
	s := slices.Clone(d)
	slices.Sort(s)
	return s[int(float64(len(s)-1)*q)]
}

// escenario levanta un servidor con `montar(backend)` y lo ataca con `clientes` clientes en bucle
// cerrado (cada uno lanza la siguiente solicitud al recibir la respuesta) durante `dur`.
// Los clientes se rinden a los 300 ms (como un balanceador o un navegador) y, tras un 503, esperan 20 ms.
func escenario(montar func(b *Backend) http.Handler, clientes int, dur time.Duration) resultado {
	b := NuevoBackend(8, 20*time.Millisecond) // capacidad ideal: 8 / 20 ms = 400 operaciones/s
	srv := NuevoServidor(montar(b))
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	ctx, parar := context.WithCancel(context.Background())
	hecho := make(chan error, 1)
	go func() { hecho <- Servir(ctx, srv, ln, 5*time.Second) }()

	cli := &http.Client{
		Timeout:   300 * time.Millisecond,
		Transport: &http.Transport{MaxIdleConnsPerHost: clientes, MaxConnsPerHost: 0},
	}
	url := "http://" + ln.Addr().String()
	var mu sync.Mutex
	var res resultado
	var pico atomic.Int64
	var wg sync.WaitGroup
	fin := time.Now().Add(dur)

	vigia := make(chan struct{})
	go func() { // muestrea el nº de goroutines del proceso
		t := time.NewTicker(10 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				pico.Store(max(pico.Load(), int64(runtime.NumGoroutine())))
			case <-vigia:
				return
			}
		}
	}()

	t0 := time.Now()
	for range clientes {
		wg.Go(func() {
			for time.Now().Before(fin) {
				t := time.Now()
				resp, err := cli.Get(url)
				d := time.Since(t)
				mu.Lock()
				switch {
				case err != nil:
					res.caducados++
				case resp.StatusCode == http.StatusOK:
					res.ok++
					res.lat = append(res.lat, d)
				default:
					res.rechazados++
				}
				mu.Unlock()
				if err == nil {
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
					if resp.StatusCode != http.StatusOK {
						time.Sleep(20 * time.Millisecond)
					}
				}
			}
		})
	}
	wg.Wait()
	res.rps = float64(res.ok) / time.Since(t0).Seconds()
	close(vigia)
	cli.CloseIdleConnections()
	parar()
	<-hecho
	res.trabajoInutil = b.Completadas.Load() - int64(res.ok)
	res.picoGoroutines = int(pico.Load())
	return res
}

func main() {
	const clientes = 200
	const dur = 2 * time.Second
	fmt.Printf("backend: 8 plazas × 20 ms ⇒ capacidad 400 op/s. %d clientes en bucle durante %v; los clientes se rinden a los 300 ms.\n\n", clientes, dur)
	fmt.Printf("  %-44s %6s %9s %9s %9s %9s %9s %s\n", "configuración", "ok/s", "rechaz.", "caducad.", "p50 ok", "p99 ok", "inútil", "goroutines (pico)")
	for _, c := range []struct {
		nombre string
		montar func(*Backend) http.Handler
	}{
		{"A) sin límite, el handler ignora ctx", func(b *Backend) http.Handler { return HandlerIgnoraCtx(b) }},
		{"B) sin límite, el handler respeta r.Context()", func(b *Backend) http.Handler { return HandlerConCtx(b) }},
		{"C) límite 8 + 503 inmediato", func(b *Backend) http.Handler { return Limitar(8, HandlerConCtx(b)) }},
		{"D) límite 8 + cola 16 (espera máx. 100 ms)", func(b *Backend) http.Handler {
			return LimitarConCola(8, 16, 100*time.Millisecond, HandlerConCtx(b))
		}},
	} {
		r := escenario(c.montar, clientes, dur)
		fmt.Printf("  %-44s %6.0f %9d %9d %9v %9v %9d %d\n", c.nombre, r.rps, r.rechazados, r.caducados,
			p(r.lat, .5).Round(time.Millisecond), p(r.lat, .99).Round(time.Millisecond), r.trabajoInutil, r.picoGoroutines)
	}
	fmt.Println("\n'inútil' = operaciones del backend terminadas cuya respuesta no recibió ningún cliente (ya se había rendido).")
}
