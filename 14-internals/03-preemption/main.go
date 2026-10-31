// Ejercicio 3: preemption cooperativa frente a asíncrona.
//
//	go run . -modo=comparar     # qué pasa con y sin preemption asíncrona
//	go run . -modo=latencia     # cuánto tarda en despertar una goroutine cuando otra acapara el P
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"slices"
	"strings"
	"time"
)

// hojaPequena: función hoja con marco diminuto. El compilador OMITE su comprobación de pila
// (el prólogo "morestack"), que es donde se detecta una petición de preemption cooperativa.
//
//go:noinline
func hojaPequena(x int) int { return x + 1 }

// conMarcoGrande: su marco supera el umbral (StackSmall = 128 B), así que su prólogo SÍ
// comprueba la pila y es un punto de preemption cooperativa.
//
//go:noinline
func conMarcoGrande(x int) int {
	var buf [512]byte
	buf[x&511] = byte(x)
	return x + int(buf[(x+1)&511]) + 1
}

// acaparar ocupa el único P:
//   - "bucle":   bucle sin llamadas
//   - "hoja":    llama en cada vuelta a una función hoja pequeña (sin comprobación de pila)
//   - "marco":   llama en cada vuelta a una función con marco grande (con comprobación de pila)
func acaparar(variante string) {
	x := 0
	switch variante {
	case "bucle":
		for {
			x++
		}
	case "hoja":
		for {
			x = hojaPequena(x)
		}
	case "marco":
		for {
			x = conMarcoGrande(x)
		}
	}
}

// hijo: ejecuta el experimento y escribe el resultado.
func hijo(variante string) {
	runtime.GOMAXPROCS(1)
	go acaparar(variante)
	time.Sleep(time.Millisecond) // deja que el acaparador empiece
	ini := time.Now()
	time.Sleep(20 * time.Millisecond) // main debería despertar tras ~20 ms
	fmt.Printf("main despertó tras %v\n", time.Since(ini).Round(time.Millisecond))
}

func comparar() {
	fmt.Printf("%-52s %s\n", "configuración (GOMAXPROCS=1, un acaparador)", "resultado")
	for _, c := range []struct{ desc, godebug, variante string }{
		{"bucle sin llamadas, por defecto", "", "bucle"},
		{"bucle sin llamadas, asyncpreemptoff=1", "asyncpreemptoff=1", "bucle"},
		{"llama a función hoja pequeña, por defecto", "", "hoja"},
		{"llama a función hoja pequeña, asyncpreemptoff=1", "asyncpreemptoff=1", "hoja"},
		{"llama a función de marco grande, por defecto", "", "marco"},
		{"llama a función de marco grande, asyncpreemptoff=1", "asyncpreemptoff=1", "marco"},
	} {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		cmd := exec.CommandContext(ctx, os.Args[0], "-modo=hijo", "-variante="+c.variante)
		cmd.Env = append(os.Environ(), "GODEBUG="+c.godebug)
		out, err := cmd.Output()
		expiro := errors.Is(ctx.Err(), context.DeadlineExceeded) // ANTES de cancel(): después siempre sería Canceled
		cancel()
		res := strings.TrimSpace(string(out))
		switch {
		case expiro:
			res = "BLOQUEADO: main nunca despertó (hubo que matar el proceso a los 3 s)"
		case err != nil:
			res = "error al ejecutar el hijo: " + err.Error()
		}
		fmt.Printf("%-52s %s\n", c.desc, res)
	}
}

// latencia mide cuánto TARDA EN DESPERTAR time.Sleep(1ms) con y sin acaparador.
func latencia() {
	runtime.GOMAXPROCS(1)
	medir := func() []time.Duration {
		var ds []time.Duration
		for range 30 {
			ini := time.Now()
			time.Sleep(time.Millisecond)
			ds = append(ds, time.Since(ini))
		}
		slices.Sort(ds)
		return ds
	}
	informe := func(nombre string, ds []time.Duration) {
		fmt.Printf("%-28s mín=%-8v mediana=%-8v máx=%-8v\n", nombre,
			ds[0].Round(10*time.Microsecond), ds[len(ds)/2].Round(10*time.Microsecond), ds[len(ds)-1].Round(10*time.Microsecond))
	}
	fmt.Println("time.Sleep(1ms) medido 30 veces, GOMAXPROCS=1:")
	informe("sin acaparador", medir())
	go acaparar("bucle")
	time.Sleep(5 * time.Millisecond)
	informe("con un acaparador (bucle)", medir())
	fmt.Println("(forcePreemptNS = 10 ms: el sondeo de sysmon expropia a quien lleva más de 10 ms en el P)")
}

func main() {
	modo := flag.String("modo", "comparar", "comparar|latencia|hijo")
	variante := flag.String("variante", "bucle", "(interno)")
	flag.Parse()
	switch *modo {
	case "comparar":
		comparar()
	case "latencia":
		latencia()
	case "hijo":
		hijo(*variante)
	}
}
