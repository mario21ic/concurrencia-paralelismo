// Caso 4.3 (avanzado): apagar un servicio sin perder trabajo ni entrar en pánico.
//
// Problema: un servicio recibe tareas y las ejecuta con N workers. Al recibir SIGTERM (o al terminar el
// programa) hay que: 1) dejar de aceptar tareas nuevas, 2) terminar las ya aceptadas, 3) no esperar
// más de una GRACIA, y 4) no provocar `send on closed channel` si alguien envía justo mientras se cierra.
//
// Herramientas: close(channel) para avisar a los workers, RWMutex para que «cerrar» y «enviar» no se
// pisen, WaitGroup para esperar, context para cancelar lo que no acabe en la gracia.
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

var (
	ErrCerrado = errors.New("servicio cerrado")
	ErrLlena   = errors.New("cola llena")
)

type Tarea func(ctx context.Context)

type Servicio struct {
	mu      sync.RWMutex // protege `cerrado` y el envío a `cola` frente al close(cola)
	cerrado bool
	cola    chan Tarea
	wg      sync.WaitGroup
	cancel  context.CancelFunc
	Hechas  atomic.Int64
}

func Iniciar(workers, capacidadCola int) *Servicio {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Servicio{cola: make(chan Tarea, capacidadCola), cancel: cancel}
	for range workers {
		s.wg.Go(func() {
			for t := range s.cola { // termina cuando la cola está CERRADA Y VACÍA: drena lo pendiente
				t(ctx)
				s.Hechas.Add(1)
			}
		})
	}
	return s
}

// Enviar encola sin bloquear. El RLock impide que Cerrar haga close(cola) en mitad del envío.
func (s *Servicio) Enviar(t Tarea) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cerrado {
		return ErrCerrado
	}
	select {
	case s.cola <- t:
		return nil
	default:
		return ErrLlena
	}
}

// Cerrar deja de aceptar tareas, espera hasta `gracia` a que terminen las pendientes y, si no acaban,
// cancela las que estén en curso. Es idempotente. Al volver no queda ninguna goroutine del servicio.
func (s *Servicio) Cerrar(gracia time.Duration) (completo bool) {
	s.mu.Lock()
	if !s.cerrado {
		s.cerrado = true
		close(s.cola) // seguro: nadie puede estar enviando (tenemos el cerrojo en exclusiva)
	}
	s.mu.Unlock()

	hecho := make(chan struct{})
	go func() { s.wg.Wait(); close(hecho) }()
	t := time.NewTimer(gracia)
	defer t.Stop()
	select {
	case <-hecho:
		return true
	case <-t.C:
		s.cancel() // se agotó la gracia: las tareas en curso ven ctx.Done()
		<-hecho
		return false
	}
}

func main() {
	s := Iniciar(2, 100)
	for i := range 6 {
		s.Enviar(func(ctx context.Context) {
			select {
			case <-time.After(100 * time.Millisecond):
			case <-ctx.Done():
				fmt.Printf("  tarea %d cancelada por la gracia\n", i)
			}
		})
	}
	t0 := time.Now()
	completo := s.Cerrar(time.Second)
	fmt.Printf("cierre con gracia suficiente: completo=%v, hechas=%d de 6, tardó %v\n", completo, s.Hechas.Load(), time.Since(t0).Round(10*time.Millisecond))
	fmt.Println("enviar tras cerrar →", s.Enviar(func(context.Context) {}))

	s2 := Iniciar(2, 100)
	for i := range 4 {
		s2.Enviar(func(ctx context.Context) {
			select {
			case <-time.After(5 * time.Second): // tareas muy largas
			case <-ctx.Done():
				fmt.Printf("  tarea %d cancelada por la gracia\n", i)
			}
		})
	}
	t0 = time.Now()
	completo = s2.Cerrar(200 * time.Millisecond)
	fmt.Printf("cierre con gracia insuficiente: completo=%v, tardó %v\n", completo, time.Since(t0).Round(10*time.Millisecond))
}
