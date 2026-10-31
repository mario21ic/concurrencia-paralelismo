// Package main (ejercicio 9): diseño de APIs concurrentes — quién lanza la goroutine y quién la detiene.
package main

import (
	"context"
	"fmt"
	"iter"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

// =====================================================================================
// 1. ¿Quién lanza la goroutine? — constructor que lanza vs. método bloqueante Run(ctx)
// =====================================================================================

// RelojMal lanza su goroutine en el constructor y no ofrece forma de pararla: quien lo crea
// no controla su ciclo de vida (fuga en cada instancia; imposible de probar de forma determinista).
type RelojMal struct {
	ticks atomic.Int64
	parar chan struct{} // solo lo usan los tests para limpiar; la API pública no lo expone
}

func NuevoRelojMal(cada time.Duration) *RelojMal {
	r := &RelojMal{parar: make(chan struct{})}
	go func() {
		t := time.NewTicker(cada)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				r.ticks.Add(1)
			case <-r.parar:
				return
			}
		}
	}()
	return r
}
func (r *RelojMal) Ticks() int64 { return r.ticks.Load() }

// Reloj no lanza nada: Run BLOQUEA hasta que ctx se cancela. El llamante decide cómo ejecutarlo
// (go, errgroup, en main…), cómo pararlo (ctx) y recibe el error. Es la convención de http.Server.Serve.
type Reloj struct {
	cada  time.Duration
	ticks atomic.Int64
}

func NuevoReloj(cada time.Duration) *Reloj { return &Reloj{cada: cada} }

func (r *Reloj) Run(ctx context.Context) error {
	t := time.NewTicker(r.cada)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			r.ticks.Add(1)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
func (r *Reloj) Ticks() int64 { return r.ticks.Load() }

// =====================================================================================
// 2. Servicio con ciclo de vida propio: Iniciar + Close que ESPERA
// =====================================================================================

// Servicio lanza una goroutine interna, pero devuelve un asa que permite pararla y esperar su
// fin: Close cancela, espera y devuelve el error de la tarea. Es idempotente y seguro de llamar
// desde varias goroutines. Úsalo cuando el componente DEBE tener goroutines internas.
type Servicio struct {
	cancel context.CancelCauseFunc
	hecho  chan struct{}
	err    error
	cierre sync.Once
}

var ErrCerrado = fmt.Errorf("servicio cerrado")

func Iniciar(ctx context.Context, tarea func(ctx context.Context) error) *Servicio {
	ctx, cancel := context.WithCancelCause(ctx)
	s := &Servicio{cancel: cancel, hecho: make(chan struct{})}
	go func() {
		defer close(s.hecho)
		defer cancel(nil)
		s.err = recuperar(func() error { return tarea(ctx) })
	}()
	return s
}

// Close pide parar y espera. Devuelve el error de la tarea (nil si salió por la cancelación de Close).
func (s *Servicio) Close() error {
	s.cierre.Do(func() { s.cancel(ErrCerrado) })
	<-s.hecho
	if s.err == ErrCerrado || s.err == context.Canceled {
		return nil
	}
	return s.err
}

// Hecho se cierra cuando la tarea ha terminado (por Close, por error o por cancelación del padre).
func (s *Servicio) Hecho() <-chan struct{} { return s.hecho }

// recuperar convierte un pánico de una goroutine hija en un error (un pánico no recuperado mataría el proceso).
func recuperar(f func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("pánico: %v", r)
		}
	}()
	return f()
}

// =====================================================================================
// 3. APIs de flujo: channel que respeta ctx, o iterador sin goroutines
// =====================================================================================

// DuplicarMal devuelve un channel SIN buffer y envía sin mirar si alguien lee: si el consumidor
// se va antes de tiempo, la goroutine queda bloqueada para siempre.
func DuplicarMal(entrada []int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for _, v := range entrada {
			out <- v * 2
		}
	}()
	return out
}

// Duplicar: la goroutine pertenece a la función, que garantiza su fin: todo envío compite con
// ctx.Done(), y el productor cierra el channel (quien envía, cierra).
func Duplicar(ctx context.Context, entrada []int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for _, v := range entrada {
			select {
			case out <- v * 2:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out
}

// DuplicarIter es la forma más segura: un iterador (iter.Seq) es SÍNCRONO. No hay goroutine
// que pueda fugarse; si el consumidor hace break, yield devuelve false y el bucle termina.
func DuplicarIter(entrada []int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for _, v := range entrada {
			if !yield(v * 2) {
				return
			}
		}
	}
}

// =====================================================================================
// 4. Concurrencia estructurada: todo lo que se lanza termina antes de que la función devuelva
// =====================================================================================

type Descarga func(ctx context.Context, url string) (string, error)

// DescargarMal lanza una goroutine por URL y devuelve en el PRIMER error sin esperar al resto:
// las demás siguen ejecutándose tras el return (fuga), escriben en `res` cuando el llamante ya
// la usa (carrera) y nadie las cancela.
func DescargarMal(urls []string, f Descarga) ([]string, error) {
	res := make([]string, len(urls))
	errc := make(chan error, 1)
	for i, u := range urls {
		go func() {
			v, err := f(context.Background(), u)
			if err != nil {
				select {
				case errc <- err:
				default:
				}
				return
			}
			res[i] = v
		}()
	}
	select {
	case err := <-errc:
		return res, err
	case <-time.After(time.Second): // "ya habrán acabado"… (hope-based design)
		return res, nil
	}
}

// DescargarTodas es la versión estructurada:
//   - el errgroup deriva un ctx que se cancela al primer error (o al cancelar el padre);
//   - SetLimit acota la concurrencia (backpressure en el lanzamiento);
//   - Wait garantiza que TODAS las goroutines terminaron al devolver: ninguna fuga, ninguna carrera;
//   - los pánicos de las hijas se convierten en errores.
func DescargarTodas(ctx context.Context, urls []string, limite int, f Descarga) ([]string, error) {
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(limite)
	res := make([]string, len(urls)) // cada goroutine escribe SOLO su índice: sin cerrojo
	for i, u := range urls {
		g.Go(func() error {
			return recuperar(func() error {
				v, err := f(ctx, u)
				if err != nil {
					return fmt.Errorf("%s: %w", u, err)
				}
				res[i] = v
				return nil
			})
		})
	}
	return res, g.Wait()
}

// DescargarWG es la misma idea con sync.WaitGroup.Go (Go 1.25): sin errgroup, sin límite, el primer
// error se guarda con sync.Once y cancela a las demás.
func DescargarWG(ctx context.Context, urls []string, f Descarga) ([]string, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	res := make([]string, len(urls))
	var wg sync.WaitGroup
	for i, u := range urls {
		wg.Go(func() {
			v, err := f(ctx, u)
			if err != nil {
				cancel(fmt.Errorf("%s: %w", u, err)) // solo la primera causa se conserva
				return
			}
			res[i] = v
		})
	}
	wg.Wait()
	return res, context.Cause(ctx)
}
