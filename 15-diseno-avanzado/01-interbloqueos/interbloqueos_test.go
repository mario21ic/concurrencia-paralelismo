package main

import (
	"sync"
	"testing"
	"time"
)

// terminaEn ejecuta f y dice si acabó antes del plazo (una goroutine bloqueada para siempre se abandona).
func terminaEn(plazo time.Duration, f func()) bool {
	hecho := make(chan struct{})
	go func() { f(); close(hecho) }()
	select {
	case <-hecho:
		return true
	case <-time.After(plazo):
		return false
	}
}

func cruzadas(f func(o, d *Cuenta)) (a, b *Cuenta) {
	a, b = &Cuenta{ID: 1, Saldo: 100}, &Cuenta{ID: 2, Saldo: 100}
	var wg sync.WaitGroup
	wg.Go(func() { f(a, b) })
	wg.Go(func() { f(b, a) })
	wg.Wait()
	return
}

func TestTransferirMalSeInterbloquea(t *testing.T) {
	if terminaEn(300*time.Millisecond, func() {
		cruzadas(func(o, d *Cuenta) { TransferirMal(o, d, 10, 20*time.Millisecond) })
	}) {
		t.Fatal("se esperaba un interbloqueo y las transferencias terminaron")
	}
}

func TestTransferirOrdenadoNoSeInterbloquea(t *testing.T) {
	var a, b *Cuenta
	if !terminaEn(2*time.Second, func() {
		a, b = cruzadas(func(o, d *Cuenta) { TransferirOrdenado(o, d, 10, 20*time.Millisecond) })
	}) {
		t.Fatal("interbloqueo con cerrojos ordenados")
	}
	if a.Saldo+b.Saldo != 200 {
		t.Fatalf("se perdió dinero: %d + %d", a.Saldo, b.Saldo)
	}
}

func TestTransferirConPlazoNoSeBloqueaYConservaSaldos(t *testing.T) {
	a, b := &Cuenta{ID: 1, Saldo: 100}, &Cuenta{ID: 2, Saldo: 100}
	var okA, okB bool
	if !terminaEn(2*time.Second, func() {
		var wg sync.WaitGroup
		wg.Go(func() { okA = TransferirConPlazo(a, b, 10, 150*time.Millisecond, 5*time.Millisecond) })
		wg.Go(func() { okB = TransferirConPlazo(b, a, 10, 150*time.Millisecond, 5*time.Millisecond) })
		wg.Wait()
	}) {
		t.Fatal("TransferirConPlazo se bloqueó")
	}
	// Ambas vencen su plazo casi a la vez: si una aborta un instante antes, suelta su cerrojo y
	// la otra puede colarse. Lo que NO puede pasar es que ambas triunfen, ni que se pierda dinero.
	if okA && okB {
		t.Fatal("ambas transferencias cruzadas no pueden haber triunfado con la pausa de 150 ms")
	}
	if a.Saldo+b.Saldo != 200 {
		t.Fatalf("se perdió dinero: %d + %d", a.Saldo, b.Saldo)
	}
	if !okA && !okB && (a.Saldo != 100 || b.Saldo != 100) {
		t.Fatalf("un aborto no debe modificar saldos: %d %d", a.Saldo, b.Saldo)
	}
}

func TestBackoffAleatorioReduceFallos(t *testing.T) {
	sin, _ := Reintentos(false, 8)
	con, _ := Reintentos(true, 8)
	t.Logf("fallos: sin backoff %d, con backoff %d", sin, con)
	if con*5 > sin {
		t.Fatalf("el backoff aleatorio debería reducir mucho los fallos (sin=%d con=%d)", sin, con)
	}
}

func TestHerenciaDePrioridadAcortaLaInversion(t *testing.T) {
	sin := Inversion(false, 10)
	con := Inversion(true, 10)
	t.Logf("sin herencia %v, con herencia %v", sin, con)
	if con*2 > sin {
		t.Fatalf("la herencia debería al menos reducir a la mitad la espera (sin=%v con=%v)", sin, con)
	}
}

func TestEnvejecimientoEvitaInanicion(t *testing.T) {
	sin := Inanicion(0, 4, 40)
	con := Inanicion(5*time.Millisecond, 4, 40)
	t.Logf("sin envejecimiento %v, con %v", sin, con)
	if con*2 > sin {
		t.Fatalf("el envejecimiento debería acortar mucho la espera (sin=%v con=%v)", sin, con)
	}
}

func TestPrioSemRespetaPrioridad(t *testing.T) {
	s := NuevoPrioSem(1)
	var yo Prio
	s.Acquire(&yo) // ocupa el único hueco
	orden := make(chan string, 2)
	var baja, alta Prio
	baja.Store(1)
	alta.Store(9)
	var wg sync.WaitGroup
	wg.Go(func() { s.Acquire(&baja); orden <- "baja"; s.Release() })
	time.Sleep(20 * time.Millisecond)
	wg.Go(func() { s.Acquire(&alta); orden <- "alta"; s.Release() })
	time.Sleep(20 * time.Millisecond)
	s.Release()
	wg.Wait()
	if p := <-orden; p != "alta" {
		t.Fatalf("primero debía entrar la alta, entró %s", p)
	}
}
