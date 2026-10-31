package main

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Barberia modela el problema del barbero dormilón con channels:
//
//   - la sala de espera es un channel con buffer = nº de sillas (lleno ⇒ el cliente se va);
//   - un barbero "duerme" bloqueado en la recepción del channel y "despierta" al llegar un cliente;
//   - cada cliente espera en su propio channel `hecho` a que le corten el pelo.
type Barberia struct {
	sala     chan *cliente
	corte    time.Duration
	barberos sync.WaitGroup

	Atendidos, Perdidos atomic.Int64
	mu                  sync.Mutex
	esperas             []time.Duration // tiempo desde que se sienta hasta que empieza el corte
}

type cliente struct {
	llegada time.Time
	hecho   chan struct{}
}

// Abrir pone a trabajar a `barberos` barberos, con `sillas` en la sala de espera.
func Abrir(ctx context.Context, barberos, sillas int, corte time.Duration) *Barberia {
	b := &Barberia{sala: make(chan *cliente, sillas), corte: corte}
	for range barberos {
		b.barberos.Go(func() {
			for {
				select {
				case c := <-b.sala: // duerme aquí hasta que llega alguien
					b.mu.Lock()
					b.esperas = append(b.esperas, time.Since(c.llegada))
					b.mu.Unlock()
					time.Sleep(b.corte)
					close(c.hecho)
				case <-ctx.Done():
					return
				}
			}
		})
	}
	return b
}

// Llega intenta entrar: si no hay silla libre, se va (devuelve false) sin esperar.
func (b *Barberia) Llega() (atendido bool) {
	c := &cliente{llegada: time.Now(), hecho: make(chan struct{})}
	select {
	case b.sala <- c:
		<-c.hecho
		b.Atendidos.Add(1)
		return true
	default:
		b.Perdidos.Add(1)
		return false
	}
}

// Cerrar espera a que los barberos terminen (tras cancelar el contexto de Abrir).
func (b *Barberia) Cerrar() { b.barberos.Wait() }

// EsperaMedia/Maxima en la sala (sin contar el corte).
func (b *Barberia) Esperas() (media, maxima time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, e := range b.esperas {
		media += e
		maxima = max(maxima, e)
	}
	if n := len(b.esperas); n > 0 {
		media /= time.Duration(n)
	}
	return
}
