package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// OpcionesSitio configura el sitio sintético.
type OpcionesSitio struct {
	Paginas    int           // número de páginas /p/0 .. /p/N-1
	Latencia   time.Duration // retardo base de cada respuesta
	Especiales bool          // activa páginas problemáticas (ver abajo)
}

// Sitio es un servidor httptest con un grafo de páginas enlazadas:
// la página i enlaza a (2i+1), (2i+2) e (i+1), módulo N, más un enlace externo
// y uno duplicado.
//
// Con Especiales:
//
//	/p/5  falla con 500 la primera vez y luego responde bien (inestable)
//	/p/7  tarda 400 ms (lenta)
//	/p/13 responde 404
//	/p/21 responde siempre 500
type Sitio struct {
	*httptest.Server
	opts OpcionesSitio

	mu    sync.Mutex
	hits  map[string]int
	actua atomic.Int32
	maxim atomic.Int32
}

func NuevoSitio(o OpcionesSitio) *Sitio {
	if o.Paginas <= 0 {
		o.Paginas = 60
	}
	s := &Sitio{opts: o, hits: make(map[string]int)}
	s.Server = httptest.NewServer(http.HandlerFunc(s.servir))
	return s
}

// Hits devuelve cuántas peticiones recibió cada ruta.
func (s *Sitio) Hits() map[string]int {
	s.mu.Lock()
	defer s.mu.Unlock()
	m := make(map[string]int, len(s.hits))
	for k, v := range s.hits {
		m[k] = v
	}
	return m
}

// MaxSimultaneas es el máximo de peticiones en curso observado en el SERVIDOR.
func (s *Sitio) MaxSimultaneas() int { return int(s.maxim.Load()) }

func (s *Sitio) servir(w http.ResponseWriter, r *http.Request) {
	a := s.actua.Add(1)
	defer s.actua.Add(-1)
	for m := s.maxim.Load(); a > m && !s.maxim.CompareAndSwap(m, a); m = s.maxim.Load() {
	}

	s.mu.Lock()
	s.hits[r.URL.Path]++
	veces := s.hits[r.URL.Path]
	s.mu.Unlock()

	select {
	case <-time.After(s.opts.Latencia):
	case <-r.Context().Done():
		return
	}

	if !strings.HasPrefix(r.URL.Path, "/p/") {
		http.NotFound(w, r)
		return
	}
	i, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/p/"))
	if err != nil || i < 0 || i >= s.opts.Paginas {
		http.NotFound(w, r)
		return
	}

	if s.opts.Especiales {
		switch i {
		case 5:
			if veces == 1 {
				http.Error(w, "inestable", http.StatusInternalServerError)
				return
			}
		case 7:
			select {
			case <-time.After(400 * time.Millisecond):
			case <-r.Context().Done():
				return
			}
		case 13:
			http.NotFound(w, r)
			return
		case 21:
			http.Error(w, "roto", http.StatusInternalServerError)
			return
		}
	}

	n := s.opts.Paginas
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<html><body><h1>Página %d</h1>
<a href="/p/%d">a</a> <a href="/p/%d">b</a> <a href="/p/%d#seccion">c</a>
<a href="/p/%d">duplicado</a> <a href="http://externo.invalid/x">externo</a>
<a href="mailto:a@b.c">correo</a></body></html>`, i, (2*i+1)%n, (2*i+2)%n, (i+1)%n, (2*i+1)%n)
}
