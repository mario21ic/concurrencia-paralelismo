package main

import (
	"sync"
	"time"
)

const cuanto = 2 * time.Millisecond // lo que dura cada "turno de CPU"

func trabajar(cpu *PrioSem, p *Prio, cuantos int) {
	for range cuantos {
		cpu.Acquire(p)
		time.Sleep(cuanto)
		cpu.Release()
	}
}

// Inversion ejecuta el escenario clásico y devuelve cuánto tarda la tarea URGENTE.
//
//	baja (prio 1): toma el cerrojo y necesita 5 turnos de CPU para soltarlo.
//	medias (prio 5): `medias` tareas de 5 turnos cada una, siempre por delante de la baja.
//	alta (prio 10): necesita el cerrojo y luego 1 turno.
//
// Sin herencia, la alta espera a la baja, y la baja espera a que TODAS las medias acaben.
func Inversion(herencia bool, medias int) time.Duration {
	cpu := NuevoPrioSem(1)
	mu := &Mutex{Herencia: herencia}
	var baja, media, alta Prio
	baja.Store(1)
	media.Store(5)
	alta.Store(10)

	bajaTieneCerrojo := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		mu.Lock(&baja)
		close(bajaTieneCerrojo)
		trabajar(cpu, &baja, 5)
		mu.Unlock()
	})
	<-bajaTieneCerrojo
	for range medias {
		wg.Go(func() { trabajar(cpu, &media, 5) })
	}
	time.Sleep(cuanto) // deja que las medias se pongan en cola

	t0 := time.Now()
	mu.Lock(&alta)
	trabajar(cpu, &alta, 1)
	mu.Unlock()
	d := time.Since(t0)
	wg.Wait()
	return d
}

// Inanicion mide cuánto espera una tarea de baja prioridad que solo necesita UN turno mientras
// `flujos` tareas urgentes piden turnos sin parar. Con envejecimiento > 0 acaba cediéndole el turno.
func Inanicion(envejecimiento time.Duration, flujos, turnosCadaUno int) time.Duration {
	cpu := NuevoPrioSem(1)
	cpu.Envejecimiento = envejecimiento
	var alta, baja Prio
	alta.Store(10)
	baja.Store(1)
	var wg sync.WaitGroup
	for range flujos {
		wg.Go(func() { trabajar(cpu, &alta, turnosCadaUno) })
	}
	time.Sleep(cuanto)
	t0 := time.Now()
	trabajar(cpu, &baja, 1)
	d := time.Since(t0)
	wg.Wait()
	return d
}
