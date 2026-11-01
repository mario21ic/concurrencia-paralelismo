// Caso 5.2 (intermedio): un pool de workers con cola acotada y decisión explícita ante la saturación.
//
// Problema: un servicio recibe más trabajo del que puede procesar. Hay tres políticas posibles cuando
// la cola está llena, y elegir una ES una decisión de diseño:
//
//	Bloquear   (Enviar con ctx) → frena al productor: contrapresión. Bueno entre componentes internos.
//	Rechazar   (Intentar)       → falla rápido: bueno en el borde (HTTP 503) para no acumular latencia.
//	Descartar  (no implementado: tirar el más viejo/el más barato) → telemetría, vídeo en directo.
//
// A diferencia del caso 4.3 (ciclo de vida y apagado), aquí el foco es la ENTRADA: workers fijos
// (acota la concurrencia) + cola acotada (acota la memoria y la latencia).
//
//	go run .
package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var ErrLlena = errors.New("pool saturado")

type Pool struct {
	cola   chan func()
	wg     sync.WaitGroup
	Hechas atomic.Int64
}

func NuevoPool(workers, capacidad int) *Pool {
	p := &Pool{cola: make(chan func(), capacidad)}
	for range workers {
		p.wg.Go(func() {
			for t := range p.cola {
				t()
				p.Hechas.Add(1)
			}
		})
	}
	return p
}

// Enviar BLOQUEA hasta que haya sitio o ctx se cancele (contrapresión hacia el llamante).
func (p *Pool) Enviar(ctx context.Context, t func()) error {
	select {
	case p.cola <- t:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Intentar NO bloquea: false si la cola está llena (el llamante decide: responder 503, descartar…).
func (p *Pool) Intentar(t func()) bool {
	select {
	case p.cola <- t:
		return true
	default:
		return false
	}
}

// Cerrar espera a que se procese todo lo aceptado. Solo debe llamarlo el dueño del pool, una vez,
// cuando ya nadie envía (para un cierre concurrente seguro, véase el caso 4.3).
func (p *Pool) Cerrar() { close(p.cola); p.wg.Wait() }

func main() {
	tarea := func() { time.Sleep(20 * time.Millisecond) }

	// Rechazar: 3 workers + cola de 5. Al enviar «de golpe», los workers apenas han empezado: entran las 5
	// de la cola y, como mucho, las pocas que los workers vayan tomando mientras tanto (varía por ejecución).
	p := NuevoPool(3, 5)
	aceptadas, rechazadas := 0, 0
	for range 20 {
		if p.Intentar(tarea) {
			aceptadas++
		} else {
			rechazadas++
		}
	}
	p.Cerrar()
	fmt.Printf("Rechazar: de 20 envíos instantáneos → %d aceptadas, %d rechazadas (503). Ejecutadas: %d\n", aceptadas, rechazadas, p.Hechas.Load())

	// Bloquear: el productor se frena al ritmo del pool, no se pierde nada.
	p = NuevoPool(3, 5)
	t0 := time.Now()
	for range 20 {
		p.Enviar(context.Background(), tarea)
	}
	enviado := time.Since(t0)
	p.Cerrar()
	fmt.Printf("Bloquear: 20 envíos aceptados todos; el productor tardó %v en enviar (frenado), todo terminó a los %v\n",
		enviado.Round(10*time.Millisecond), time.Since(t0).Round(10*time.Millisecond))
}
