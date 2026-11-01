// Caso 3.2 (intermedio): una cadena de etapas conectadas por channels (pipeline) con cancelación.
//
// Problema: procesar un flujo de líneas de log: leer → parsear → agregar. Cada etapa es una goroutine
// que lee de la anterior y escribe en la siguiente. Se pueden solapar (mientras se agrega la línea 1 ya
// se parsea la 2), y todo se detiene limpiamente si se cancela el contexto.
//
// Regla de diseño: TODO envío compite con ctx.Done(); si no, una etapa queda bloqueada para siempre
// cuando el consumidor se va (fuga de goroutines).
//
//	go run .
package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync/atomic"
)

type Registro struct {
	Servicio string
	MS       int
}

type Resumen struct{ N, TotalMS int }

// Lineas emite cada línea (etapa fuente).
func Lineas(ctx context.Context, lineas []string) <-chan string {
	out := make(chan string)
	go func() {
		defer close(out)
		for _, l := range lineas {
			select {
			case out <- l:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

// Parsear convierte "servicio|ms" en Registro y descarta las líneas malformadas (contándolas).
func Parsear(ctx context.Context, in <-chan string, invalidas *atomic.Int64) <-chan Registro {
	out := make(chan Registro)
	go func() {
		defer close(out)
		for l := range in {
			svc, msTxt, ok := strings.Cut(l, "|")
			ms, err := strconv.Atoi(msTxt)
			if !ok || err != nil || svc == "" {
				invalidas.Add(1)
				continue
			}
			select {
			case out <- Registro{svc, ms}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

// Resumir agrega por servicio (etapa sumidero: devuelve cuando la anterior cierra).
func Resumir(in <-chan Registro) map[string]Resumen {
	res := map[string]Resumen{}
	for r := range in {
		x := res[r.Servicio]
		x.N++
		x.TotalMS += r.MS
		res[r.Servicio] = x
	}
	return res
}

// Ejecutar monta el pipeline completo.
func Ejecutar(ctx context.Context, lineas []string) (map[string]Resumen, int64) {
	var invalidas atomic.Int64
	res := Resumir(Parsear(ctx, Lineas(ctx, lineas), &invalidas))
	return res, invalidas.Load()
}

func main() {
	lineas := []string{"api|120", "db|40", "api|80", "mala", "db|60", "api|x", "cache|5"}
	res, inval := Ejecutar(context.Background(), lineas)
	fmt.Printf("resumen: %v | líneas inválidas: %d\n", res, inval)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelado de antemano: el pipeline debe terminar sin colgarse ni dejar goroutines
	res, _ = Ejecutar(ctx, lineas)
	fmt.Printf("con ctx cancelado: %v (resultado parcial; terminó limpiamente)\n", res)
}
