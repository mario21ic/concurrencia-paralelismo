// Mini-proyecto 3: crawler web concurrente.
//
//	semilla ──► programar(url) ──► [errgroup: una goroutine por URL]
//	                                   │
//	             límite de concurrencia (semáforo) + límite de tasa (token bucket)
//	                                   │
//	                       descargar (timeout por petición, reintentos)
//	                                   │
//	                 resultados  +  enlaces nuevos ──► programar(...)
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

// Config agrupa todos los límites del crawler.
type Config struct {
	MaxConcurrencia int           // peticiones simultáneas (defecto 8)
	Rate            float64       // peticiones por segundo (0 = sin límite)
	Burst           int           // ráfaga permitida por el limitador (defecto 1)
	Timeout         time.Duration // plazo de CADA petición (defecto 5 s)
	MaxPaginas      int           // páginas a visitar como máximo (defecto 1000)
	MaxProfundidad  int           // 0 = solo la semilla
	Reintentos      int           // reintentos ante error de red/timeout/5xx
	Espera          time.Duration // espera base entre reintentos (backoff exponencial)
	MismoHost       bool          // no salir del host de la semilla
	Cliente         *http.Client  // opcional
}

func (c *Config) normalizar() {
	if c.MaxConcurrencia <= 0 {
		c.MaxConcurrencia = 8
	}
	if c.Burst <= 0 {
		c.Burst = 1
	}
	if c.Timeout <= 0 {
		c.Timeout = 5 * time.Second
	}
	if c.MaxPaginas <= 0 {
		c.MaxPaginas = 1000
	}
	if c.Espera <= 0 {
		c.Espera = 50 * time.Millisecond
	}
	if c.Cliente == nil {
		c.Cliente = &http.Client{}
	}
}

// Pagina es el resultado de visitar una URL.
type Pagina struct {
	URL         string
	Profundidad int
	Estado      int // 0 si no hubo respuesta
	Bytes       int
	Enlaces     int
	Intentos    int
	Duracion    time.Duration
	Err         error
}

// Informe resume el recorrido.
type Informe struct {
	Paginas        []Pagina
	Duracion       time.Duration
	MaxSimultaneas int
}

func (i *Informe) Contar() (ok, fallos int) {
	for _, p := range i.Paginas {
		if p.Err == nil {
			ok++
		} else {
			fallos++
		}
	}
	return
}

// Ordenadas devuelve las páginas ordenadas por URL (determinista).
func (i *Informe) Ordenadas() []Pagina {
	ps := slices.Clone(i.Paginas)
	slices.SortFunc(ps, func(a, b Pagina) int { return strings.Compare(a.URL, b.URL) })
	return ps
}

type crawler struct {
	cfg  Config
	base *url.URL
	sem  chan struct{}
	lim  *rate.Limiter

	mu          sync.Mutex
	vistas      map[string]struct{}
	programadas int
	paginas     []Pagina

	actuales, maxActuales atomic.Int32
}

// Crawl recorre el sitio desde `semilla`. Devuelve SIEMPRE el informe parcial;
// el error es no nil solo si ctx se canceló o la semilla es inválida.
func Crawl(ctx context.Context, semilla string, cfg Config) (*Informe, error) {
	cfg.normalizar()
	base, err := url.Parse(semilla)
	if err != nil || (base.Scheme != "http" && base.Scheme != "https") || base.Host == "" {
		return nil, fmt.Errorf("semilla inválida: %q", semilla)
	}
	c := &crawler{
		cfg:    cfg,
		base:   base,
		sem:    make(chan struct{}, cfg.MaxConcurrencia),
		vistas: make(map[string]struct{}),
	}
	if cfg.Rate > 0 {
		c.lim = rate.NewLimiter(rate.Limit(cfg.Rate), cfg.Burst)
	}

	ini := time.Now()
	g, gctx := errgroup.WithContext(ctx)
	c.programar(gctx, g, base.String(), 0)
	err = g.Wait()

	return &Informe{Paginas: c.paginas, Duracion: time.Since(ini), MaxSimultaneas: int(c.maxActuales.Load())}, err
}

// programar registra la URL (una sola vez) y lanza su visita.
// Es seguro llamarlo desde dentro de una goroutine del propio grupo: mientras
// esa goroutine siga viva, Wait no puede haber terminado.
func (c *crawler) programar(ctx context.Context, g *errgroup.Group, u string, prof int) {
	c.mu.Lock()
	if _, ya := c.vistas[u]; ya || c.programadas >= c.cfg.MaxPaginas {
		c.mu.Unlock()
		return
	}
	c.vistas[u] = struct{}{}
	c.programadas++
	c.mu.Unlock()

	g.Go(func() error { return c.visitar(ctx, g, u, prof) })
}

func (c *crawler) visitar(ctx context.Context, g *errgroup.Group, u string, prof int) error {
	// Límite de concurrencia: solo MaxConcurrencia descargas a la vez.
	select {
	case c.sem <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	pag, enlaces := c.descargar(ctx, u, prof)
	<-c.sem // se libera ANTES de encolar los enlaces: no se retiene la plaza sin necesidad

	c.mu.Lock()
	c.paginas = append(c.paginas, pag)
	c.mu.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}
	if prof >= c.cfg.MaxProfundidad {
		return nil
	}
	for _, e := range enlaces {
		if c.cfg.MismoHost {
			if eu, err := url.Parse(e); err != nil || eu.Host != c.base.Host {
				continue
			}
		}
		c.programar(ctx, g, e, prof+1)
	}
	return nil
}

// descargar hace la petición con límite de tasa, timeout propio y reintentos.
func (c *crawler) descargar(ctx context.Context, u string, prof int) (Pagina, []string) {
	pag := Pagina{URL: u, Profundidad: prof}
	ini := time.Now()
	var cuerpo string

	for intento := 0; ; intento++ {
		pag.Intentos = intento + 1

		if c.lim != nil { // cada INTENTO consume un token
			if err := c.lim.Wait(ctx); err != nil {
				pag.Err = err
				break
			}
		}

		estado, texto, err := c.peticion(ctx, u)
		pag.Estado, pag.Err = estado, err
		if err == nil && estado >= 400 {
			pag.Err = fmt.Errorf("HTTP %d", estado)
		}
		cuerpo = texto

		reintentable := (err != nil && ctx.Err() == nil) || estado >= 500
		if !reintentable || intento >= c.cfg.Reintentos {
			break
		}
		espera := c.cfg.Espera << intento // backoff exponencial
		select {
		case <-time.After(espera):
		case <-ctx.Done():
			pag.Err = ctx.Err()
			pag.Duracion = time.Since(ini)
			return pag, nil
		}
	}
	pag.Duracion = time.Since(ini)
	pag.Bytes = len(cuerpo)

	if pag.Err != nil || pag.Estado != http.StatusOK {
		return pag, nil
	}
	base, err := url.Parse(u)
	if err != nil {
		return pag, nil
	}
	enlaces := ExtraerEnlaces(base, cuerpo)
	pag.Enlaces = len(enlaces)
	return pag, enlaces
}

// peticion hace UNA petición con su propio plazo y mide la concurrencia real.
func (c *crawler) peticion(ctx context.Context, u string) (estado int, cuerpo string, err error) {
	ctx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, "", err
	}
	a := c.actuales.Add(1)
	defer c.actuales.Add(-1)
	for m := c.maxActuales.Load(); a > m && !c.maxActuales.CompareAndSwap(m, a); m = c.maxActuales.Load() {
	}

	resp, err := c.cfg.Cliente.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // máx. 1 MB por página
	if err != nil && !errors.Is(err, io.EOF) {
		return resp.StatusCode, "", err
	}
	return resp.StatusCode, string(b), nil
}
