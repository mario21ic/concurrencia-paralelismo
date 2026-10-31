// Ejercicio 6: cómo se implementan los channels (hchan) y select, visto desde fuera.
//
//	go run .
package main

import (
	"fmt"
	"runtime"
	"sort"
	"sync"
	"time"
)

func memoria() uint64 {
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

func main() {
	// ---- A) Memoria: un hchan fijo + el buffer circular en el mismo bloque ----
	fmt.Println("== A) memoria de un channel: cabecera hchan (~96 B) + buffer circular ==")
	const n = 1000
	for _, c := range []struct {
		nombre string
		make   func() any
		elem   int
	}{
		{"chan struct{}  (sin buffer)", func() any { return make(chan struct{}) }, 0},
		{"chan int       (buffer 1000)", func() any { return make(chan int, 1000) }, 8},
		{"chan [64]byte  (buffer 1000)", func() any { return make(chan [64]byte, 1000) }, 64},
	} {
		base := memoria()
		chs := make([]any, n)
		for i := range chs {
			chs[i] = c.make()
		}
		uso := memoria() - base
		fmt.Printf("  %-30s %8.0f bytes por channel", c.nombre, float64(uso)/n)
		if c.elem > 0 {
			fmt.Printf("  (buffer teórico %d B)", c.elem*1000)
		}
		fmt.Println()
		runtime.KeepAlive(chs)
	}

	// ---- B) Los bloqueados esperan en una cola FIFO (sudog) ----
	fmt.Println("\n== B) receptores bloqueados: cola FIFO (recvq) ==")
	ch := make(chan int)
	orden := make(chan int, 8)
	for i := range 8 {
		go func() { <-ch; orden <- i }()
		time.Sleep(5 * time.Millisecond) // garantiza que se bloquean en orden i = 0..7
	}
	for i := range 8 {
		ch <- i
		time.Sleep(time.Millisecond)
	}
	var salida []int
	for range 8 {
		salida = append(salida, <-orden)
	}
	fmt.Printf("  bloqueados en el orden 0..7 → despertados en el orden %v\n", salida)
	fmt.Println("  Cada envío entrega el valor DIRECTO al primer receptor de recvq (sin pasar por buffer).")

	// ---- C) select elige al azar entre los casos listos ----
	fmt.Println("\n== C) select con varios casos listos: elección pseudoaleatoria uniforme ==")
	a, b, c := make(chan int, 1), make(chan int, 1), make(chan int, 1)
	cuenta := map[string]int{}
	const rondas = 300_000
	for range rondas {
		a <- 0
		b <- 0
		c <- 0
		select {
		case <-a:
			cuenta["a"]++
			<-b
			<-c
		case <-b:
			cuenta["b"]++
			<-a
			<-c
		case <-c:
			cuenta["c"]++
			<-a
			<-b
		}
	}
	for _, k := range []string{"a", "b", "c"} {
		fmt.Printf("  %s: %5.2f %%\n", k, 100*float64(cuenta[k])/rondas)
	}
	fmt.Println("  Sin orden de prioridad: si necesitas prioridad, anida un select con default.")

	// ---- D) Coste: el select bloquea (lock) todos los channels implicados ----
	fmt.Println("\n== D) coste de select según el nº de casos (solo el último caso está listo) ==")
	for _, k := range []int{1, 2, 4, 8} {
		chs := make([]chan int, k)
		for i := range chs {
			chs[i] = make(chan int, 1)
		}
		ns := medirSelect(chs)
		fmt.Printf("  %2d casos: %6.1f ns por select\n", k, ns)
	}
	fmt.Println("  select ordena los channels por dirección, los bloquea todos, elige uno y los desbloquea:")
	fmt.Println("  el coste crece con el nº de casos. Con 1 caso + default el compilador lo convierte en una operación directa.")

	// ---- E) Coste channel sin buffer / con buffer / mutex ----
	fmt.Println("\n== E) pasar un valor entre dos goroutines (ping) ==")
	fmt.Printf("  sin buffer, ping-pong:         %6.0f ns por viaje de ida y vuelta\n", pingPong(0))
	fmt.Printf("  con buffer (1000), productor→consumidor: %6.0f ns por elemento\n", flujo(1000))
	fmt.Printf("  sin buffer, productor→consumidor:        %6.0f ns por elemento\n", flujo(0))
	fmt.Printf("  mutex + contador (mismo nº ops):         %6.0f ns por operación\n", conMutex())

	// ---- F) nil bloquea para siempre; cerrado nunca bloquea ----
	fmt.Println("\n== F) channels especiales dentro de un select ==")
	var nulo chan int
	cerrado := make(chan int)
	close(cerrado)
	listos := map[string]int{}
	for range 1000 {
		select {
		case <-nulo:
			listos["nil"]++
		case v, ok := <-cerrado:
			_ = v
			if !ok {
				listos["cerrado"]++
			}
		}
	}
	keys := make([]string, 0, len(listos))
	for k := range listos {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("  %-8s elegido %d veces de 1000\n", k, listos[k])
	}
	fmt.Println("  Un channel nil nunca está listo (se usa para desactivar casos); uno cerrado siempre lo está.")
}

func medirSelect(chs []chan int) float64 {
	const rep = 1_000_000
	// Rellenamos el último; los demás vacíos: el select recorre todos y encuentra el listo.
	ult := chs[len(chs)-1]
	ult <- 0
	t0 := time.Now()
	for range rep {
		switch len(chs) {
		case 1:
			select {
			case v := <-chs[0]:
				chs[0] <- v
			}
		case 2:
			select {
			case v := <-chs[0]:
				chs[0] <- v
			case v := <-chs[1]:
				chs[1] <- v
			}
		default:
			selectN(chs)
		}
	}
	return float64(time.Since(t0).Nanoseconds()) / rep
}

// selectN ejecuta un select con 4 u 8 casos fijos.
func selectN(chs []chan int) {
	switch len(chs) {
	case 4:
		select {
		case v := <-chs[0]:
			chs[0] <- v
		case v := <-chs[1]:
			chs[1] <- v
		case v := <-chs[2]:
			chs[2] <- v
		case v := <-chs[3]:
			chs[3] <- v
		}
	case 8:
		select {
		case v := <-chs[0]:
			chs[0] <- v
		case v := <-chs[1]:
			chs[1] <- v
		case v := <-chs[2]:
			chs[2] <- v
		case v := <-chs[3]:
			chs[3] <- v
		case v := <-chs[4]:
			chs[4] <- v
		case v := <-chs[5]:
			chs[5] <- v
		case v := <-chs[6]:
			chs[6] <- v
		case v := <-chs[7]:
			chs[7] <- v
		}
	}
}

func pingPong(buf int) float64 {
	const rep = 200_000
	ida, vuelta := make(chan int, buf), make(chan int, buf)
	go func() {
		for v := range ida {
			vuelta <- v
		}
		close(vuelta)
	}()
	t0 := time.Now()
	for i := range rep {
		ida <- i
		<-vuelta
	}
	d := time.Since(t0)
	close(ida)
	return float64(d.Nanoseconds()) / rep
}

func flujo(buf int) float64 {
	const rep = 2_000_000
	ch := make(chan int, buf)
	var wg sync.WaitGroup
	wg.Go(func() {
		for range ch {
		}
	})
	t0 := time.Now()
	for i := range rep {
		ch <- i
	}
	close(ch)
	wg.Wait()
	return float64(time.Since(t0).Nanoseconds()) / rep
}

func conMutex() float64 {
	const rep = 2_000_000
	var mu sync.Mutex
	var x int
	var wg sync.WaitGroup
	t0 := time.Now()
	for range 2 {
		wg.Go(func() {
			for range rep / 2 {
				mu.Lock()
				x++
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	return float64(time.Since(t0).Nanoseconds()) / rep
}
