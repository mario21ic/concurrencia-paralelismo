// Package traza resume un archivo de traza de ejecución (runtime/trace) sin
// necesidad de navegador: tiempo de las goroutines por estado, motivos de
// espera, regiones y tareas definidas por el usuario.
package traza

import (
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/exp/trace"
)

// Resumen es el resultado del análisis.
type Resumen struct {
	Eventos     int
	Duracion    time.Duration
	Goroutines  int                      // goroutines distintas observadas
	PorEstado   map[string]time.Duration // tiempo acumulado por estado (Running, Runnable, Waiting...)
	PorMotivo   map[string]time.Duration // tiempo bloqueado, por motivo de espera
	Regiones    map[string]*Estadistica  // regiones del usuario
	Tareas      map[string]*Estadistica
	Logs        int
	MaxRunnable int // máximo de goroutines listas simultáneamente (indicador de saturación)
}

// Estadistica acumula duraciones de una región o tarea.
type Estadistica struct {
	N     int
	Total time.Duration
	Max   time.Duration
}

func (e *Estadistica) anadir(d time.Duration) {
	e.N++
	e.Total += d
	e.Max = max(e.Max, d)
}

type estadoG struct {
	estado string
	motivo string
	desde  trace.Time
}

type claveRegion struct {
	g    trace.GoID
	tipo string
}

// Analizar lee la traza completa.
func Analizar(r io.Reader) (*Resumen, error) {
	lector, err := trace.NewReader(r)
	if err != nil {
		return nil, err
	}
	res := &Resumen{
		PorEstado: map[string]time.Duration{},
		PorMotivo: map[string]time.Duration{},
		Regiones:  map[string]*Estadistica{},
		Tareas:    map[string]*Estadistica{},
	}
	gs := map[trace.GoID]*estadoG{}
	regiones := map[claveRegion][]trace.Time{}
	tareas := map[trace.TaskID]struct {
		tipo  string
		desde trace.Time
	}{}
	var primero, ultimo trace.Time
	var tieneInicio bool
	runnable := 0

	for {
		ev, err := lector.ReadEvent()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		res.Eventos++
		t := ev.Time()
		if !tieneInicio {
			primero, tieneInicio = t, true
		}
		ultimo = t

		switch ev.Kind() {
		case trace.EventStateTransition:
			st := ev.StateTransition()
			if st.Resource.Kind != trace.ResourceGoroutine {
				continue
			}
			id := st.Resource.Goroutine()
			desde, hasta := st.Goroutine()

			if previo, ok := gs[id]; ok {
				d := t.Sub(previo.desde)
				res.PorEstado[previo.estado] += d
				if previo.estado == "Waiting" {
					res.PorMotivo[previo.motivo] += d
				}
			} else {
				res.Goroutines++
			}
			if desde.String() == "Runnable" {
				runnable--
			}
			if hasta.String() == "Runnable" {
				runnable++
				res.MaxRunnable = max(res.MaxRunnable, runnable)
			}
			if hasta.String() == "NotExist" {
				delete(gs, id)
				continue
			}
			gs[id] = &estadoG{estado: hasta.String(), motivo: st.Reason, desde: t}

		case trace.EventRegionBegin:
			k := claveRegion{ev.Goroutine(), ev.Region().Type}
			regiones[k] = append(regiones[k], t)
		case trace.EventRegionEnd:
			k := claveRegion{ev.Goroutine(), ev.Region().Type}
			if pila := regiones[k]; len(pila) > 0 {
				ini := pila[len(pila)-1]
				regiones[k] = pila[:len(pila)-1]
				acum(res.Regiones, k.tipo).anadir(t.Sub(ini))
			}
		case trace.EventTaskBegin:
			tk := ev.Task()
			tareas[tk.ID] = struct {
				tipo  string
				desde trace.Time
			}{tk.Type, t}
		case trace.EventTaskEnd:
			tk := ev.Task()
			if ini, ok := tareas[tk.ID]; ok {
				acum(res.Tareas, ini.tipo).anadir(t.Sub(ini.desde))
				delete(tareas, tk.ID)
			}
		case trace.EventLog:
			res.Logs++
		}
	}
	if tieneInicio {
		res.Duracion = ultimo.Sub(primero)
	}
	return res, nil
}

func acum(m map[string]*Estadistica, k string) *Estadistica {
	if m[k] == nil {
		m[k] = &Estadistica{}
	}
	return m[k]
}

// Imprimir escribe un informe legible.
func (r *Resumen) Imprimir(w io.Writer, topMotivos int) {
	fmt.Fprintf(w, "eventos: %d | duración de la traza: %v | goroutines observadas: %d | máx. goroutines listas a la vez: %d\n",
		r.Eventos, r.Duracion.Round(time.Millisecond), r.Goroutines, r.MaxRunnable)

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "\nTIEMPO ACUMULADO DE GOROUTINES POR ESTADO\t")
	for _, k := range clavesPorValor(r.PorEstado) {
		fmt.Fprintf(tw, "  %s\t%v\n", k, r.PorEstado[k].Round(time.Millisecond))
	}
	tw.Flush()

	if len(r.PorMotivo) > 0 {
		fmt.Fprintln(w, "\nESPERAS (Waiting) POR MOTIVO")
		for i, k := range clavesPorValor(r.PorMotivo) {
			if i >= topMotivos {
				break
			}
			fmt.Fprintf(w, "  %-34s %v\n", k, r.PorMotivo[k].Round(time.Millisecond))
		}
	}
	imprimirEst(w, "REGIONES", r.Regiones)
	imprimirEst(w, "TAREAS", r.Tareas)
	if r.Logs > 0 {
		fmt.Fprintf(w, "\nmensajes trace.Log: %d\n", r.Logs)
	}
}

func imprimirEst(w io.Writer, titulo string, m map[string]*Estadistica) {
	if len(m) == 0 {
		return
	}
	fmt.Fprintf(w, "\n%s\n  %-22s %8s %12s %12s %12s\n", titulo, "nombre", "veces", "total", "media", "máx.")
	claves := make([]string, 0, len(m))
	for k := range m {
		claves = append(claves, k)
	}
	slices.Sort(claves)
	for _, k := range claves {
		e := m[k]
		fmt.Fprintf(w, "  %-22s %8d %12v %12v %12v\n", k, e.N, e.Total.Round(time.Microsecond), (e.Total / time.Duration(e.N)).Round(time.Microsecond), e.Max.Round(time.Microsecond))
	}
}

func clavesPorValor(m map[string]time.Duration) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	slices.SortFunc(ks, func(a, b string) int {
		if c := int(m[b] - m[a]); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	})
	return ks
}
