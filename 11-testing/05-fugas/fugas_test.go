package fugas

import (
	"testing"
	"testing/synctest"
	"time"

	"go.uber.org/goleak"
)

// Las versiones CORRECTAS, comprobadas con tres detectores distintos.

func TestObtenerBien_Goleak(t *testing.T) {
	defer goleak.VerifyNone(t) // falla si quedan goroutines al terminar el test
	_, err := ObtenerBien(func() int { time.Sleep(50 * time.Millisecond); return 1 }, time.Millisecond)
	if err != ErrPlazo {
		t.Fatalf("err = %v", err)
	}
}

func TestObtenerBien_Helper(t *testing.T) {
	SinFugas(t)
	_, err := ObtenerBien(func() int { time.Sleep(50 * time.Millisecond); return 1 }, time.Millisecond)
	if err != ErrPlazo {
		t.Fatalf("err = %v", err)
	}
}

// Con synctest el detector es la propia burbuja: si al terminar el test queda
// una goroutine bloqueada, Test entra en pánico (deadlock).
//
// Matiz: el tiempo virtual deja de avanzar cuando termina la goroutine raíz.
// El trabajo de 1 h (virtual) sigue "pendiente" al salir, así que el test debe
// dejar pasar el tiempo para que acabe. La versión BUENA entonces termina limpia;
// la MALA (bugsynctest_test.go) sigue bloqueada enviando y provoca el pánico.
func TestObtenerBien_Synctest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		_, err := ObtenerBien(func() int { time.Sleep(time.Hour); return 1 }, time.Second)
		if err != ErrPlazo {
			t.Fatalf("err = %v", err)
		}
		time.Sleep(time.Hour) // deja que el trabajo termine: envía al buffer y sale
	})
}

func TestEscuchaCerrandoElChannel(t *testing.T) {
	defer goleak.VerifyNone(t)
	ch := make(chan int)
	hecho := make(chan struct{})
	EscuchaMal(ch, func(v int) {
		if v == 2 {
			close(hecho)
		}
	})
	ch <- 1
	ch <- 2
	<-hecho
	close(ch) // el que lo usa cierra el channel: la goroutine termina
}

func TestObtenerBien_SinTimeout(t *testing.T) {
	defer goleak.VerifyNone(t)
	v, err := ObtenerBien(func() int { return 42 }, time.Second)
	if err != nil || v != 42 {
		t.Fatalf("v=%d err=%v", v, err)
	}
}
