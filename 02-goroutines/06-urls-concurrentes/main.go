// Ejercicio 6: consultar N URLs en paralelo con WaitGroup.
//
// Usa servidores httptest locales con latencia simulada, así el ejercicio
// no depende de internet. Cada goroutine escribe en SU posición del slice
// de resultados, por lo que no hace falta mutex.
package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"
)

type resultado struct {
	url    string
	estado int
	bytes  int
	demora time.Duration
	err    error
}

func obtener(url string) resultado {
	ini := time.Now()
	resp, err := http.Get(url)
	if err != nil {
		return resultado{url: url, err: err, demora: time.Since(ini)}
	}
	defer resp.Body.Close()
	cuerpo, err := io.ReadAll(resp.Body)
	return resultado{url: url, estado: resp.StatusCode, bytes: len(cuerpo), demora: time.Since(ini), err: err}
}

func secuencial(urls []string) []resultado {
	out := make([]resultado, len(urls))
	for i, u := range urls {
		out[i] = obtener(u)
	}
	return out
}

func concurrente(urls []string) []resultado {
	out := make([]resultado, len(urls))
	var wg sync.WaitGroup
	for i, u := range urls {
		wg.Go(func() { out[i] = obtener(u) }) // índice distinto por goroutine: sin race
	}
	wg.Wait()
	return out
}

func main() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// latencia entre 50 y 250 ms según el path
		var n int
		fmt.Sscanf(r.URL.Path, "/%d", &n)
		time.Sleep(time.Duration(50+(n*37)%200) * time.Millisecond)
		if n%7 == 0 {
			http.Error(w, "fallo simulado", http.StatusInternalServerError)
			return
		}
		fmt.Fprintf(w, "respuesta %d", n)
	}))
	defer srv.Close()

	var urls []string
	for i := 1; i <= 20; i++ {
		urls = append(urls, fmt.Sprintf("%s/%d", srv.URL, i))
	}

	ini := time.Now()
	secuencial(urls)
	dSec := time.Since(ini)

	ini = time.Now()
	res := concurrente(urls)
	dCon := time.Since(ini)

	fallos := 0
	for _, r := range res {
		if r.err != nil || r.estado != 200 {
			fallos++
		}
	}
	fmt.Printf("URLs: %d, fallos HTTP: %d\n", len(urls), fallos)
	fmt.Printf("secuencial: %v\nconcurrente: %v (%.1fx)\n",
		dSec.Round(time.Millisecond), dCon.Round(time.Millisecond), float64(dSec)/float64(dCon))
	for _, r := range res[:4] {
		fmt.Printf("  %-32s estado=%d bytes=%d en %v\n", r.url, r.estado, r.bytes, r.demora.Round(time.Millisecond))
	}
}
