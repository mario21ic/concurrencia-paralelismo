// Package main (ejercicio 10): concurrencia en servidores net/http — goroutine por solicitud,
// límites, backpressure, cancelación y apagado ordenado.
package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"sync/atomic"
	"time"
)

// ---- Un "recurso caro" detrás del servidor (una base de datos con 8 conexiones, p. ej.) ----

// Backend simula un recurso de capacidad limitada: `hueco` plazas y `coste` por operación.
type Backend struct {
	hueco       chan struct{}
	coste       time.Duration
	Completadas atomic.Int64 // operaciones terminadas (útiles o no)
}

func NuevoBackend(plazas int, coste time.Duration) *Backend {
	return &Backend{hueco: make(chan struct{}, plazas), coste: coste}
}

// Operar ocupa una plaza durante `coste`. Respeta ctx tanto al esperar plaza como al trabajar.
func (b *Backend) Operar(ctx context.Context) error {
	select {
	case b.hueco <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-b.hueco }()
	t := time.NewTimer(b.coste)
	defer t.Stop()
	select {
	case <-t.C:
		b.Completadas.Add(1)
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ---- Handlers ----

// HandlerIgnoraCtx trabaja aunque el cliente se haya ido: el trabajo de solicitudes abandonadas se desperdicia.
func HandlerIgnoraCtx(b *Backend) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b.Operar(context.Background())
		w.Write([]byte("ok"))
	})
}

// HandlerConCtx propaga r.Context(): se cancela cuando el cliente cierra la conexión o vence el plazo.
func HandlerConCtx(b *Backend) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := b.Operar(r.Context()); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		w.Write([]byte("ok"))
	})
}

// ---- Middleware ----

// Limitar admite como máximo `n` solicitudes simultáneas; las demás reciben 503 + Retry-After AL INSTANTE
// (descarte de carga, *load shedding*): la latencia de las admitidas no se degrada y el cliente se entera ya.
func Limitar(n int, siguiente http.Handler) http.Handler {
	sem := make(chan struct{}, n)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case sem <- struct{}{}:
			defer func() { <-sem }()
			siguiente.ServeHTTP(w, r)
		default:
			w.Header().Set("Retry-After", "1")
			http.Error(w, "servidor saturado", http.StatusServiceUnavailable)
		}
	})
}

// LimitarConCola: `n` en ejecución + hasta `enCola` esperando como máximo `espera`; el resto, 503 inmediato.
// La cola absorbe ráfagas, pero acotada: una cola larga solo añade latencia (ejercicio 4).
func LimitarConCola(n, enCola int, espera time.Duration, siguiente http.Handler) http.Handler {
	admision := make(chan struct{}, n+enCola) // plazas totales (ejecutando + esperando)
	ejecucion := make(chan struct{}, n)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case admision <- struct{}{}:
			defer func() { <-admision }()
		default:
			w.Header().Set("Retry-After", "1")
			http.Error(w, "cola llena", http.StatusServiceUnavailable)
			return
		}
		t := time.NewTimer(espera)
		defer t.Stop()
		select {
		case ejecucion <- struct{}{}:
			defer func() { <-ejecucion }()
			siguiente.ServeHTTP(w, r)
		case <-t.C:
			w.Header().Set("Retry-After", "1")
			http.Error(w, "demasiada espera", http.StatusServiceUnavailable)
		case <-r.Context().Done(): // el cliente se fue mientras esperaba: no hay nada que hacer
		}
	})
}

// ---- Servidor con tiempos límite y apagado ordenado ----

// NuevoServidor devuelve un http.Server con los tiempos límite que protegen las goroutines por conexión
// (sin ellos, un cliente lento o malicioso retiene una goroutine y un descriptor indefinidamente).
func NuevoServidor(h http.Handler) *http.Server {
	return &http.Server{
		Handler:           h,
		ReadHeaderTimeout: 2 * time.Second,  // mitiga *slowloris* (cabeceras a goteo)
		ReadTimeout:       10 * time.Second, // cabeceras + cuerpo
		WriteTimeout:      10 * time.Second, // respuesta
		IdleTimeout:       60 * time.Second, // conexiones keep-alive ociosas
		MaxHeaderBytes:    1 << 16,
	}
}

// Servir atiende en ln hasta que ctx se cancela; entonces deja de aceptar, espera a las solicitudes
// en curso (hasta `gracia`) y devuelve. Es el patrón Run(ctx) del ejercicio 9 aplicado a HTTP.
func Servir(ctx context.Context, srv *http.Server, ln net.Listener, gracia time.Duration) error {
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case err := <-errc:
		return err // fallo al servir
	case <-ctx.Done():
	}
	paro, cancel := context.WithTimeout(context.Background(), gracia)
	defer cancel()
	if err := srv.Shutdown(paro); err != nil {
		srv.Close() // se agotó la gracia: corta lo que quede
		return err
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
