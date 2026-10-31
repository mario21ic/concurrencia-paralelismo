// Ejercicio 1: el planificador G-M-P por dentro, observado.
//
//	go run . -modo=orden        # orden de ejecución con un solo P (runnext + FIFO)
//	go run . -modo=reparto      # reparto de goroutines entre Ps (work stealing), medido con la traza
//	GODEBUG=schedtrace=1 go run . -modo=cola   # desbordamiento de la cola local a la global
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"runtime"
	"runtime/trace"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	xtrace "golang.org/x/exp/trace"
)

// ordenDeEjecucion crea n goroutines SIN ceder el procesador y anota en qué orden empiezan.
func ordenDeEjecucion(n int) string {
	var mu sync.Mutex
	var orden []string
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			mu.Lock()
			orden = append(orden, string(rune('A'+i)))
			mu.Unlock()
		})
	}
	wg.Wait() // main se bloquea aquí: recién entonces el único P empieza a ejecutar las demás
	return strings.Join(orden, " ")
}

// repartoPorP ejecuta `n` goroutines creadas por UNA sola goroutine, con la traza activa, y cuenta
// en qué P empezó a ejecutarse cada una.
func repartoPorP(n int) (map[int]int, error) {
	var buf bytes.Buffer
	if err := trace.Start(&buf); err != nil {
		return nil, err
	}
	var wg sync.WaitGroup
	var hechas atomic.Int64
	for range n {
		wg.Go(func() {
			fin := time.Now().Add(50 * time.Microsecond)
			for time.Now().Before(fin) { // 50 µs de CPU
			}
			hechas.Add(1)
		})
	}
	wg.Wait()
	trace.Stop()

	r, err := xtrace.NewReader(&buf)
	if err != nil {
		return nil, err
	}
	porP := map[int]int{}
	for {
		ev, err := r.ReadEvent()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if ev.Kind() != xtrace.EventStateTransition {
			continue
		}
		st := ev.StateTransition()
		if st.Resource.Kind != xtrace.ResourceGoroutine {
			continue
		}
		if _, a := st.Goroutine(); a == xtrace.GoRunning && ev.Proc() != xtrace.NoProc {
			porP[int(ev.Proc())]++ // "una goroutine pasó a Running en este P"
		}
	}
	return porP, nil
}

func main() {
	modo := flag.String("modo", "orden", "orden|reparto|cola")
	flag.Parse()

	switch *modo {
	case "orden":
		runtime.GOMAXPROCS(1)
		fmt.Println("GOMAXPROCS=1, 8 goroutines creadas seguidas (A..H), main se bloquea:")
		distintos := map[string]int{}
		for range 200 {
			distintos[ordenDeEjecucion(8)]++
		}
		for o, n := range distintos {
			fmt.Printf("  %s   (%d de 200 ejecuciones)\n", o, n)
		}
		fmt.Println("(la última creada ocupa la ranura runnext y va primero; el resto, en orden FIFO)")

	case "reparto":
		for _, p := range []int{1, 4, 8} {
			runtime.GOMAXPROCS(p)
			porP, err := repartoPorP(4000)
			if err != nil {
				panic(err)
			}
			ps := make([]int, 0, len(porP))
			tot := 0
			for k, v := range porP {
				ps = append(ps, k)
				tot += v
			}
			sort.Ints(ps)
			fmt.Printf("\nGOMAXPROCS=%d: 4000 goroutines creadas por UNA goroutine; %d inicios de ejecución en %d Ps\n", p, tot, len(ps))
			for _, k := range ps {
				fmt.Printf("  P%-2d %5d  %s\n", k, porP[k], strings.Repeat("█", porP[k]*40/tot))
			}
		}

	case "cola":
		// Con GODEBUG=schedtrace=1 el runtime imprime el estado del planificador cada 1 ms.
		runtime.GOMAXPROCS(1)
		for range 1000 {
			go func() { time.Sleep(time.Hour) }()
		}
		// Ocupamos el único P unos milisegundos SIN llamar al planificador, para que las 1000
		// goroutines estén listas en las colas y se vea cómo se reparten.
		fin := time.Now().Add(6 * time.Millisecond)
		for time.Now().Before(fin) {
		}
		fmt.Println("(main termina)")
	}
}
