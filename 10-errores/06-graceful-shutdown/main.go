// Ejercicio 6: apagado ordenado de un servidor HTTP.
//
//	go run .                       # escucha en :8080; Ctrl+C para apagar
//	go run . -duracion 3s          # se apaga sola a los 3 s
//	curl localhost:8080/lento      # tarda 2 s: lánzalo y pulsa Ctrl+C mientras tanto
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func manejador() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) { fmt.Fprintln(w, "pong") })
	mux.HandleFunc("/lento", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(2 * time.Second):
			fmt.Fprintln(w, "listo (tras 2s)")
		case <-r.Context().Done():
			log.Println("cliente desconectado")
		}
	})
	return mux
}

func main() {
	addr := flag.String("addr", ":8080", "dirección")
	gracia := flag.Duration("gracia", 5*time.Second, "tiempo máximo para terminar peticiones en curso")
	duracion := flag.Duration("duracion", 0, "apagarse sola tras este tiempo (0 = esperar señal)")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if *duracion > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, *duracion)
		defer cancel()
	}

	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("escuchando en %s (pid %d)", ln.Addr(), os.Getpid())

	go func() { // tras la primera señal, restaurar el comportamiento por defecto
		<-ctx.Done()
		log.Println("apagando... (otra señal fuerza la salida)")
		stop()
	}()

	if err := Ejecutar(ctx, ln, manejador(), *gracia, nil); err != nil {
		log.Fatalf("apagado con error: %v", err)
	}
	log.Println("apagado limpio")
}
