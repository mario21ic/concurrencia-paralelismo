package main

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

func TestResumenYDescarteDeInvalidas(t *testing.T) {
	res, inval := Ejecutar(t.Context(), []string{"api|120", "db|40", "api|80", "mala", "api|x", "|5"})
	if inval != 3 || res["api"] != (Resumen{2, 200}) || res["db"] != (Resumen{1, 40}) || len(res) != 2 {
		t.Fatalf("res=%v inválidas=%d", res, inval)
	}
}

// Si el consumidor abandona a mitad, la cancelación libera a todas las etapas (goleak lo comprueba).
func TestCancelarMitadDeFlujoNoDejaGoroutines(t *testing.T) {
	lineas := make([]string, 10_000)
	for i := range lineas {
		lineas[i] = fmt.Sprintf("svc|%d", i)
	}
	ctx, cancel := context.WithCancel(t.Context())
	var inval atomic.Int64
	regs := Parsear(ctx, Lineas(ctx, lineas), &inval)
	for range 5 { // lee solo 5 registros y se va
		<-regs
	}
	cancel()
	for range regs { // vaciar hasta que la etapa cierre su salida
	}
}
