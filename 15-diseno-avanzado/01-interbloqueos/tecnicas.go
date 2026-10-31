package main

import (
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"time"
)

// Cuenta es un recurso con identidad (para ordenar los cerrojos).
type Cuenta struct {
	ID    int
	mu    sync.Mutex
	Saldo int
}

// TransferirMal toma origen y luego destino: dos transferencias cruzadas se bloquean mutuamente.
// pausa fuerza el entrelazado fatal (en producción lo daría la mala suerte).
func TransferirMal(o, d *Cuenta, n int, pausa time.Duration) {
	o.mu.Lock()
	defer o.mu.Unlock()
	time.Sleep(pausa)
	d.mu.Lock()
	defer d.mu.Unlock()
	o.Saldo -= n
	d.Saldo += n
}

// TransferirOrdenado toma siempre primero la cuenta de menor ID: sin ciclo de espera no hay deadlock.
func TransferirOrdenado(o, d *Cuenta, n int, pausa time.Duration) {
	a, b := o, d
	if a.ID > b.ID {
		a, b = b, a
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	time.Sleep(pausa)
	b.mu.Lock()
	defer b.mu.Unlock()
	o.Saldo -= n
	d.Saldo += n
}

// TransferirConPlazo evita bloquearse indefinidamente: si no consigue el segundo cerrojo en
// `plazo`, suelta el primero y devuelve false (el llamante reintenta o falla). Rompe la
// condición "mantener y esperar", a costa de poder fallar.
func TransferirConPlazo(o, d *Cuenta, n int, pausa, plazo time.Duration) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	time.Sleep(pausa)
	t := time.NewTimer(plazo)
	defer t.Stop()
	for !d.mu.TryLock() {
		select {
		case <-t.C:
			return false
		case <-time.After(100 * time.Microsecond):
		}
	}
	defer d.mu.Unlock()
	o.Saldo -= n
	d.Saldo += n
	return true
}

// Reintentos cuenta los intentos fallidos de TryLock hasta que dos goroutines (que toman los
// cerrojos en orden opuesto y sueltan el primero si no consiguen el segundo) terminan.
// Sin backoff pueden "hacerse cortesías" sin progresar (livelock); con retardo aleatorio, no.
func Reintentos(backoffAleatorio bool, goroutines int) (fallos int64, d time.Duration) {
	a, b := new(sync.Mutex), new(sync.Mutex)
	var fallidos atomic.Int64
	var wg sync.WaitGroup
	t0 := time.Now()
	for i := range goroutines {
		primero, segundo := a, b
		if i%2 == 1 {
			primero, segundo = b, a
		}
		wg.Go(func() {
			for {
				primero.Lock()
				time.Sleep(50 * time.Microsecond) // trabajo con el primero; ventana para chocar
				if segundo.TryLock() {
					time.Sleep(50 * time.Microsecond) // sección crítica
					segundo.Unlock()
					primero.Unlock()
					return
				}
				primero.Unlock() // "cortesía": cede y lo vuelve a intentar al instante
				fallidos.Add(1)
				if backoffAleatorio {
					time.Sleep(time.Duration(rand.IntN(400)) * time.Microsecond)
				}
			}
		})
	}
	wg.Wait()
	return fallidos.Load(), time.Since(t0)
}
