package main

import (
	"context"
	"errors"
	"net/url"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func configBase() Config {
	return Config{MaxConcurrencia: 8, MaxPaginas: 1000, MaxProfundidad: 100, Timeout: time.Second, MismoHost: true}
}

func TestExtraerEnlaces(t *testing.T) {
	base, _ := url.Parse("http://h.com/a/b")
	html := `<a href="/x">1</a> <a href='y'>2</a> <A HREF="/x#frag">3</A>
	         <a href="mailto:a@b.c">m</a> <a href="javascript:void(0)">j</a>
	         <a href="http://otro.com/z">4</a> <a href="ftp://f/g">f</a>`
	got := ExtraerEnlaces(base, html)
	want := []string{"http://h.com/x", "http://h.com/a/y", "http://otro.com/z"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v\nwant %v", got, want)
	}
}

func TestRecorreTodoUnaVez(t *testing.T) {
	s := NuevoSitio(OpcionesSitio{Paginas: 60, Latencia: time.Millisecond})
	defer s.Close()

	inf, err := Crawl(context.Background(), s.URL+"/p/0", configBase())
	if err != nil {
		t.Fatal(err)
	}
	if len(inf.Paginas) != 60 {
		t.Fatalf("páginas=%d, want 60", len(inf.Paginas))
	}
	for ruta, n := range s.Hits() {
		if n != 1 {
			t.Errorf("%s visitada %d veces; debería ser 1 (deduplicación)", ruta, n)
		}
	}
	if ok, fallos := inf.Contar(); ok != 60 || fallos != 0 {
		t.Fatalf("ok=%d fallos=%d", ok, fallos)
	}
	for _, p := range inf.Paginas {
		if strings.Contains(p.URL, "externo.invalid") {
			t.Fatal("no debería salir del host")
		}
	}
}

func TestLimiteDeConcurrencia(t *testing.T) {
	s := NuevoSitio(OpcionesSitio{Paginas: 60, Latencia: 20 * time.Millisecond})
	defer s.Close()
	cfg := configBase()
	cfg.MaxConcurrencia = 3
	inf, err := Crawl(context.Background(), s.URL+"/p/0", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if s.MaxSimultaneas() > 3 || inf.MaxSimultaneas > 3 {
		t.Fatalf("servidor vio %d simultáneas, cliente %d; límite 3", s.MaxSimultaneas(), inf.MaxSimultaneas)
	}
	if s.MaxSimultaneas() < 2 {
		t.Fatalf("máx. simultáneas=%d: no se aprovechó la concurrencia", s.MaxSimultaneas())
	}
}

func TestLimiteDeTasa(t *testing.T) {
	s := NuevoSitio(OpcionesSitio{Paginas: 60, Latencia: time.Millisecond})
	defer s.Close()
	cfg := configBase()
	cfg.Rate, cfg.Burst, cfg.MaxPaginas = 100, 1, 21 // 21 peticiones a 100/s => >= 200 ms
	inf, _ := Crawl(context.Background(), s.URL+"/p/0", cfg)
	if len(inf.Paginas) != 21 {
		t.Fatalf("páginas=%d, want 21", len(inf.Paginas))
	}
	if inf.Duracion < 180*time.Millisecond {
		t.Fatalf("duración %v: el limitador no frenó (esperado >= ~200ms)", inf.Duracion)
	}
}

func TestMaxPaginas(t *testing.T) {
	s := NuevoSitio(OpcionesSitio{Paginas: 60})
	defer s.Close()
	cfg := configBase()
	cfg.MaxPaginas = 10
	inf, _ := Crawl(context.Background(), s.URL+"/p/0", cfg)
	if len(inf.Paginas) != 10 {
		t.Fatalf("páginas=%d, want 10", len(inf.Paginas))
	}
}

func TestProfundidad(t *testing.T) {
	s := NuevoSitio(OpcionesSitio{Paginas: 60})
	defer s.Close()
	cfg := configBase()
	cfg.MaxProfundidad = 1
	inf, _ := Crawl(context.Background(), s.URL+"/p/0", cfg)
	// /p/0 enlaza a /p/1 y /p/2 (el tercero, (0+1)%60, duplica a /p/1)
	if len(inf.Paginas) != 3 {
		t.Fatalf("páginas=%d, want 3", len(inf.Paginas))
	}
	for _, p := range inf.Paginas {
		if p.Profundidad > 1 {
			t.Fatalf("profundidad %d > 1", p.Profundidad)
		}
	}
}

func TestPaginasProblematicas(t *testing.T) {
	s := NuevoSitio(OpcionesSitio{Paginas: 60, Latencia: time.Millisecond, Especiales: true})
	defer s.Close()
	cfg := configBase()
	cfg.Timeout = 100 * time.Millisecond
	cfg.Reintentos = 1
	cfg.Espera = 5 * time.Millisecond
	inf, err := Crawl(context.Background(), s.URL+"/p/0", cfg)
	if err != nil {
		t.Fatal(err)
	}
	por := map[string]Pagina{}
	for _, p := range inf.Paginas {
		por[strings.TrimPrefix(p.URL, s.URL)] = p
	}

	if p := por["/p/5"]; p.Err != nil || p.Intentos != 2 || p.Estado != 200 {
		t.Errorf("/p/5 (inestable) debía recuperarse al reintentar: %+v", p)
	}
	if p := por["/p/13"]; p.Estado != 404 || p.Err == nil || p.Intentos != 1 {
		t.Errorf("/p/13 debía ser 404 SIN reintentos: %+v", p)
	}
	if p := por["/p/21"]; p.Estado != 500 || p.Err == nil || p.Intentos != 2 {
		t.Errorf("/p/21 debía fallar con 500 tras reintentar: %+v", p)
	}
	if p := por["/p/7"]; !errors.Is(p.Err, context.DeadlineExceeded) || p.Intentos != 2 {
		t.Errorf("/p/7 (lenta) debía agotar el plazo de la petición: %+v", p)
	}
	// el resto del sitio sigue accesible pese a los fallos
	if ok, _ := inf.Contar(); ok < 50 {
		t.Errorf("solo %d páginas correctas", ok)
	}
}

func TestCancelacionRapidaYSinFugas(t *testing.T) {
	base := runtime.NumGoroutine()
	s := NuevoSitio(OpcionesSitio{Paginas: 60, Latencia: 300 * time.Millisecond})
	ctx, cancel := context.WithTimeout(context.Background(), 80*time.Millisecond)
	defer cancel()

	ini := time.Now()
	inf, err := Crawl(ctx, s.URL+"/p/0", configBase())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v, want DeadlineExceeded", err)
	}
	if d := time.Since(ini); d > 500*time.Millisecond {
		t.Fatalf("tardó %v en detenerse", d)
	}
	if inf == nil {
		t.Fatal("debe devolver el informe parcial")
	}
	s.Close()
	s.Client().CloseIdleConnections()
	time.Sleep(150 * time.Millisecond)
	if d := runtime.NumGoroutine(); d > base+3 {
		t.Fatalf("fuga de goroutines: base=%d después=%d", base, d)
	}
}

func TestSemillaInvalida(t *testing.T) {
	for _, s := range []string{"", "no es url", "ftp://x/y", "/relativa"} {
		if _, err := Crawl(context.Background(), s, configBase()); err == nil {
			t.Errorf("semilla %q debería rechazarse", s)
		}
	}
}
