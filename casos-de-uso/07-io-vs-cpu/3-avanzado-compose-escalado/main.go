// Caso 7.3 (avanzado): paralelismo ENTRE procesos con Docker Compose.
//
// Un mismo binario con dos modos:
//
//	-modo=servidor   un "worker": HTTP GET /trabajo?iter=N hace N iteraciones de CPU pura y responde
//	-modo=cliente    reparte TAREAS peticiones entre los workers y mide cuánto tarda
//
// Con Compose, `docker compose up --scale worker=N` crea N contenedores worker (cada uno limitado a
// 1 CPU) detrás del nombre DNS "worker" (Docker lo resuelve a las N IP), y el cliente reparte las
// conexiones en round-robin (dialBalanceado). El paralelismo ya no viene de goroutines dentro de un
// proceso, sino de AÑADIR procesos.
//
// El cliente puede además dejar una fracción -serial del trabajo total en SU propio proceso (una sola
// goroutine, antes de repartir): es la parte no paralelizable de la ley de Amdahl (caso 8.4).
//
//	go run . -modo=servidor &            # un worker local
//	go run . -modo=cliente -url=http://localhost:8080 -tareas=100 -iter=2000000
//
// o, con contenedores: ver README.md y ./scripts/escalar.sh
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// Gasta es CPU pura (xorshift): ni memoria compartida ni esperas.
func Gasta(iter int, semilla uint64) uint64 {
	x := semilla | 1
	for range iter {
		x ^= x << 13
		x ^= x >> 7
		x ^= x << 17
	}
	return x
}

// Manejador es el worker: ejecuta el trabajo y dice QUIÉN lo hizo (cabecera X-Worker), para que el
// cliente pueda comprobar que la carga se reparte entre contenedores.
func Manejador(nombre string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /salud", func(w http.ResponseWriter, _ *http.Request) { fmt.Fprintln(w, "ok") })
	mux.HandleFunc("GET /trabajo", func(w http.ResponseWriter, r *http.Request) {
		iter, err1 := strconv.Atoi(r.URL.Query().Get("iter"))
		semilla, err2 := strconv.ParseUint(r.URL.Query().Get("semilla"), 10, 64)
		if err1 != nil || err2 != nil || iter < 0 {
			http.Error(w, "iter y semilla deben ser enteros", http.StatusBadRequest)
			return
		}
		w.Header().Set("X-Worker", nombre)
		fmt.Fprintf(w, "%d", Gasta(iter, semilla))
	})
	return mux
}

// Config describe la carga que genera el cliente.
type Config struct {
	URL    string
	Tareas int     // peticiones al servicio
	Iter   int     // iteraciones de CPU de CADA tarea si Serial = 0
	Conc   int     // peticiones simultáneas
	Serial float64 // fracción (0..1) del trabajo total que se hace localmente, sin repartir
}

// Resultado resume una ejecución.
type Resultado struct {
	Serial, Paralela, Total time.Duration
	PorWorker               map[string]int
	Xor                     uint64 // XOR de todas las respuestas: no depende del reparto
}

// Ejecutar hace primero la parte serial (en este proceso, una goroutine) y después reparte las tareas.
// El trabajo TOTAL es el mismo para cualquier Serial: lo que se quita de cada tarea se hace antes.
func Ejecutar(ctx context.Context, cfg Config) (Resultado, error) {
	res := Resultado{PorWorker: map[string]int{}}
	t0 := time.Now()

	total := cfg.Tareas * cfg.Iter
	res.Xor ^= Gasta(int(float64(total)*cfg.Serial), 1)
	res.Serial = time.Since(t0)

	iterTarea := int(float64(cfg.Iter) * (1 - cfg.Serial))
	// Sin keep-alive cada petición abre una conexión nueva, y dialBalanceado elige la réplica. Con
	// conexiones persistentes quedarían pegadas a las primeras réplicas.
	cli := &http.Client{Timeout: 2 * time.Minute, Transport: &http.Transport{
		DisableKeepAlives: true,
		DialContext:       dialBalanceado(&net.Dialer{Timeout: 10 * time.Second}),
	}}

	tareas := make(chan int)
	var (
		mu       sync.Mutex
		wg       sync.WaitGroup
		primerEr error
	)
	for range max(1, cfg.Conc) {
		wg.Go(func() {
			for i := range tareas {
				r, err := peticion(ctx, cli, fmt.Sprintf("%s/trabajo?iter=%d&semilla=%d", cfg.URL, iterTarea, i+2))
				mu.Lock()
				if err != nil {
					primerEr = errors.Join(primerEr, err)
				} else {
					res.PorWorker[r.worker]++
					res.Xor ^= r.valor
				}
				mu.Unlock()
			}
		})
	}
	for i := range cfg.Tareas {
		select {
		case tareas <- i:
		case <-ctx.Done():
		}
	}
	close(tareas)
	wg.Wait()
	res.Total = time.Since(t0)
	res.Paralela = res.Total - res.Serial
	return res, primerEr
}

// dialBalanceado reparte las conexiones en round-robin entre TODAS las IP a las que resuelve el nombre.
// Docker devuelve una IP por réplica de `worker`, pero el resolvedor de Go las reordena (RFC 6724) y
// siempre acabaría usando la misma: sin este reparto explícito, todas las peticiones irían a un
// único contenedor y el escalado sería nulo.
func dialBalanceado(d *net.Dialer) func(ctx context.Context, red, dir string) (net.Conn, error) {
	var n atomic.Uint64
	return func(ctx context.Context, red, dir string) (net.Conn, error) {
		host, puerto, err := net.SplitHostPort(dir)
		if err != nil {
			return nil, err
		}
		ips, err := net.DefaultResolver.LookupHost(ctx, host)
		if err != nil {
			return nil, err
		}
		sort.Strings(ips) // orden estable entre llamadas para que el contador reparta de forma pareja
		return d.DialContext(ctx, red, net.JoinHostPort(ips[int(n.Add(1))%len(ips)], puerto))
	}
}

type respuesta struct {
	worker string
	valor  uint64
}

func peticion(ctx context.Context, cli *http.Client, url string) (respuesta, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return respuesta{}, err
	}
	resp, err := cli.Do(req)
	if err != nil {
		return respuesta{}, err
	}
	defer resp.Body.Close()
	cuerpo, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return respuesta{}, fmt.Errorf("%s: %s", resp.Status, cuerpo)
	}
	v, err := strconv.ParseUint(string(cuerpo), 10, 64)
	return respuesta{worker: resp.Header.Get("X-Worker"), valor: v}, err
}

func main() {
	modo := flag.String("modo", "servidor", "servidor | cliente")
	addr := flag.String("addr", ":8080", "dirección del servidor")
	url := flag.String("url", "http://worker:8080", "URL base de los workers (cliente)")
	tareas := flag.Int("tareas", 800, "número de peticiones (cliente)")
	iter := flag.Int("iter", 10_000_000, "iteraciones de CPU por tarea si serial=0 (cliente)")
	conc := flag.Int("conc", 64, "peticiones simultáneas (cliente)")
	serial := flag.Float64("serial", 0, "fracción del trabajo que se hace sin repartir, 0..1 (cliente)")
	flag.Parse()

	ctx, parar := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer parar()

	switch *modo {
	case "servidor":
		nombre, _ := os.Hostname()
		srv := &http.Server{Addr: *addr, Handler: Manejador(nombre)}
		go func() {
			<-ctx.Done()
			sd, c := context.WithTimeout(context.Background(), 10*time.Second)
			defer c()
			srv.Shutdown(sd)
		}()
		log.Printf("worker %s escuchando en %s", nombre, *addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	case "cliente":
		cfg := Config{URL: *url, Tareas: *tareas, Iter: *iter, Conc: *conc, Serial: *serial}
		res, err := Ejecutar(ctx, cfg)
		if err != nil {
			log.Fatalf("cliente: %v", err)
		}
		fmt.Printf("cliente: %d tareas, %d iteraciones de CPU en total, %d peticiones simultáneas, parte serial %.0f %%\n",
			cfg.Tareas, cfg.Tareas*cfg.Iter, cfg.Conc, 100*cfg.Serial)
		fmt.Printf("serial %v | paralela %v | total %v | %.0f tareas/s\n",
			res.Serial.Round(time.Millisecond), res.Paralela.Round(time.Millisecond), res.Total.Round(time.Millisecond),
			float64(cfg.Tareas)/res.Total.Seconds())
		nombres := make([]string, 0, len(res.PorWorker))
		for n := range res.PorWorker {
			nombres = append(nombres, n)
		}
		sort.Strings(nombres)
		fmt.Printf("reparto entre %d workers:", len(nombres))
		for _, n := range nombres {
			fmt.Printf(" %s=%d", n, res.PorWorker[n])
		}
		fmt.Println()
		// línea fácil de parsear desde los scripts
		fmt.Printf("RESULTADO total_ms=%d workers=%d\n", res.Total.Milliseconds(), len(nombres))
	default:
		log.Fatalf("modo desconocido %q", *modo)
	}
}
