// Mini-proyecto 3: crawler concurrente.
//
//	go run .                                  # recorre un sitio sintético local
//	go run . -conc 2 -rate 20                 # menos concurrencia y límite de tasa
//	go run . -url https://ejemplo.com -max 20 # un sitio real (¡sé respetuoso!)
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"
)

func main() {
	url := flag.String("url", "", "URL semilla (vacío = sitio sintético local)")
	conc := flag.Int("conc", 8, "peticiones simultáneas")
	tasa := flag.Float64("rate", 0, "peticiones por segundo (0 = sin límite)")
	timeout := flag.Duration("timeout", 150*time.Millisecond, "plazo de cada petición")
	max := flag.Int("max", 100, "máximo de páginas")
	prof := flag.Int("prof", 10, "profundidad máxima")
	reint := flag.Int("reintentos", 1, "reintentos por página")
	latencia := flag.Duration("latencia", 20*time.Millisecond, "latencia del sitio sintético")
	flag.Parse()

	semilla := *url
	if semilla == "" {
		sitio := NuevoSitio(OpcionesSitio{Paginas: 60, Latencia: *latencia, Especiales: true})
		defer sitio.Close()
		semilla = sitio.URL + "/p/0"
		fmt.Println("sitio sintético en", sitio.URL, "(60 páginas; /p/5 inestable, /p/7 lenta, /p/13 → 404, /p/21 → 500)")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	informe, err := Crawl(ctx, semilla, Config{
		MaxConcurrencia: *conc,
		Rate:            *tasa,
		Burst:           1,
		Timeout:         *timeout,
		MaxPaginas:      *max,
		MaxProfundidad:  *prof,
		Reintentos:      *reint,
		Espera:          20 * time.Millisecond,
		MismoHost:       true,
	})
	if informe == nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	ok, fallos := informe.Contar()
	fmt.Printf("\n%d páginas en %v (%d correctas, %d con error) | máx. simultáneas: %d\n",
		len(informe.Paginas), informe.Duracion.Round(time.Millisecond), ok, fallos, informe.MaxSimultaneas)
	if err != nil {
		fmt.Println("interrumpido:", err)
	}

	fmt.Println("\npáginas con error:")
	for _, p := range informe.Ordenadas() {
		if p.Err != nil {
			fmt.Printf("  %-40s estado=%d intentos=%d err=%v\n", p.URL, p.Estado, p.Intentos, p.Err)
		}
	}

	lentas := slices.Clone(informe.Paginas)
	slices.SortFunc(lentas, func(a, b Pagina) int { return int(b.Duracion - a.Duracion) })
	fmt.Println("\n3 más lentas:")
	for _, p := range lentas[:min(3, len(lentas))] {
		fmt.Printf("  %-40s %v\n", p.URL, p.Duracion.Round(time.Millisecond))
	}
}
