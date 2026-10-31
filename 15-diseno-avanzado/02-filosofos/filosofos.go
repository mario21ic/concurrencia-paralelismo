package main

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Estrategia decide en qué orden y bajo qué reglas se toman los tenedores.
type Estrategia int

const (
	Ingenua    Estrategia = iota // izquierdo y luego derecho: puede interbloquearse
	Ordenada                     // siempre primero el tenedor de menor número (rompe la espera circular)
	Mayordomo                    // como mucho n-1 filósofos sentados a la vez (rompe el ciclo completo)
	Asimetrica                   // los pares empiezan por la derecha (rompe la simetría)
)

func (e Estrategia) String() string {
	return [...]string{"ingenua", "ordenada", "mayordomo (n-1)", "asimétrica"}[e]
}

// tenedor es un cerrojo CANCELABLE: un channel de capacidad 1 (lleno = ocupado).
type tenedor chan struct{}

func (t tenedor) tomar(ctx context.Context) bool {
	select {
	case t <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}
func (t tenedor) soltar() { <-t }

// Resultado resume una cena.
type Resultado struct {
	Comidas      []int64       // veces que comió cada filósofo
	UltimaComida time.Duration // cuándo se produjo la última comida (desde el inicio)
}

func (r Resultado) Total() (t int64) {
	for _, c := range r.Comidas {
		t += c
	}
	return
}

// Equidad devuelve min/max de las comidas (1 = reparto perfecto, 0 = alguien no comió).
func (r Resultado) Equidad() float64 {
	mn, mx := r.Comidas[0], r.Comidas[0]
	for _, c := range r.Comidas {
		mn, mx = min(mn, c), max(mx, c)
	}
	if mx == 0 {
		return 0
	}
	return float64(mn) / float64(mx)
}

// Cena sienta a n filósofos durante `dur`. `pausa` es el retardo entre coger el primer y el segundo
// tenedor (en un sistema real lo pone la mala suerte del planificador: aquí lo forzamos).
func Cena(e Estrategia, n int, dur, comer, pausa time.Duration) Resultado {
	ctx, cancel := context.WithTimeout(context.Background(), dur)
	defer cancel()

	tenedores := make([]tenedor, n)
	for i := range tenedores {
		tenedores[i] = make(tenedor, 1)
	}
	var mayordomo chan struct{}
	if e == Mayordomo {
		mayordomo = make(chan struct{}, n-1)
	}

	comidas := make([]atomic.Int64, n)
	var ultima atomic.Int64
	t0 := time.Now()
	var wg sync.WaitGroup
	for i := range n {
		izq, der := i, (i+1)%n
		primero, segundo := izq, der
		switch e {
		case Ordenada:
			primero, segundo = min(izq, der), max(izq, der)
		case Asimetrica:
			if i%2 == 0 {
				primero, segundo = der, izq
			}
		}
		wg.Go(func() {
			for ctx.Err() == nil {
				if mayordomo != nil {
					select {
					case mayordomo <- struct{}{}:
					case <-ctx.Done():
						return
					}
				}
				if !tenedores[primero].tomar(ctx) {
					return
				}
				time.Sleep(pausa)
				if !tenedores[segundo].tomar(ctx) {
					tenedores[primero].soltar()
					return
				}
				time.Sleep(comer)
				comidas[i].Add(1)
				ultima.Store(int64(time.Since(t0)))
				tenedores[segundo].soltar()
				tenedores[primero].soltar()
				if mayordomo != nil {
					<-mayordomo
				}
			}
		})
	}
	wg.Wait()
	r := Resultado{Comidas: make([]int64, n), UltimaComida: time.Duration(ultima.Load())}
	for i := range comidas {
		r.Comidas[i] = comidas[i].Load()
	}
	return r
}
