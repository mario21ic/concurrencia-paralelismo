// Package lotes: un acumulador que vuelca sus elementos por tamaño, por tiempo
// o al cerrarse. Sirve para comparar tests basados en Sleep con tests
// deterministas.
package lotes

import "time"

// Acumulador agrupa enteros y llama a `volcar` con cada lote.
//
// El "tiempo" llega por un channel inyectado (`tick`): en producción se pasa
// time.NewTicker(...).C; en los tests, un channel controlado a mano.
type Acumulador struct {
	entrada chan int
	cerrar  chan struct{}
	hecho   chan struct{}
}

func Nuevo(tam int, tick <-chan time.Time, volcar func([]int)) *Acumulador {
	a := &Acumulador{
		entrada: make(chan int),
		cerrar:  make(chan struct{}),
		hecho:   make(chan struct{}),
	}
	go a.bucle(tam, tick, volcar)
	return a
}

func (a *Acumulador) bucle(tam int, tick <-chan time.Time, volcar func([]int)) {
	defer close(a.hecho)
	var buf []int
	vaciar := func() {
		if len(buf) > 0 {
			volcar(buf)
			buf = nil
		}
	}
	for {
		select {
		case v := <-a.entrada:
			buf = append(buf, v)
			if len(buf) >= tam {
				vaciar()
			}
		case <-tick:
			vaciar()
		case <-a.cerrar:
			vaciar()
			return
		}
	}
}

// Agregar envía v. Cuando retorna, el bucle ya lo recibió (channel sin buffer).
func (a *Acumulador) Agregar(v int) { a.entrada <- v }

// Cerrar vuelca lo pendiente y espera a que termine la goroutine.
func (a *Acumulador) Cerrar() {
	close(a.cerrar)
	<-a.hecho
}
