package main

import (
	"context"
	"errors"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

func servicios(recom func(context.Context) ([]string, error), pedidos func(context.Context) ([]string, error)) Servicios {
	return Servicios{
		Usuario:         tardar(50*time.Millisecond, "ana", nil),
		Pedidos:         pedidos,
		Recomendaciones: recom,
	}
}

var ped = tardar(80*time.Millisecond, []string{"#1"}, nil)

func TestTodoBien(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		t0 := time.Now()
		p, err := Armar(t.Context(), servicios(tardar(60*time.Millisecond, []string{"x"}, nil), ped), time.Second, 200*time.Millisecond)
		if err != nil || len(p.Incompleto) != 0 || p.Usuario != "ana" || len(p.Recomendaciones) != 1 || time.Since(t0) != 80*time.Millisecond {
			t.Fatalf("%+v err=%v tras %v", p, err, time.Since(t0))
		}
	})
}

func TestOpcionalLentoNoRetrasaAlPanelMasDeSuPlazo(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		t0 := time.Now()
		p, err := Armar(t.Context(), servicios(tardar(5*time.Second, []string{"x"}, nil), ped), time.Second, 200*time.Millisecond)
		if err != nil || len(p.Incompleto) != 1 || !strings.Contains(p.Incompleto[0], "deadline") {
			t.Fatalf("%+v err=%v", p, err)
		}
		if d := time.Since(t0); d != 200*time.Millisecond {
			t.Fatalf("tardó %v, esperado el plazo del opcional (200 ms)", d)
		}
		if p.Usuario != "ana" || len(p.Pedidos) != 1 {
			t.Fatal("los esenciales deben estar presentes")
		}
	})
}

func TestOpcionalQueFallaSoloSeAnota(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, err := Armar(t.Context(), servicios(tardar[[]string](10*time.Millisecond, nil, errors.New("503")), ped), time.Second, 200*time.Millisecond)
		if err != nil || len(p.Incompleto) != 1 || p.Pedidos == nil {
			t.Fatalf("%+v err=%v", p, err)
		}
	})
}

func TestEsencialQueFallaAbortaTodoYCancelaAlOpcional(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		boom := errors.New("BD caída")
		t0 := time.Now()
		_, err := Armar(t.Context(), servicios(tardar[[]string](5*time.Second, nil, nil), tardar[[]string](20*time.Millisecond, nil, boom)), time.Second, 2*time.Second)
		if !errors.Is(err, boom) || time.Since(t0) != 20*time.Millisecond {
			t.Fatalf("err=%v tras %v: debía abortar a los 20 ms", err, time.Since(t0))
		}
	})
}

func TestPlazoGlobalLimitaAlPanel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		_, err := Armar(t.Context(), servicios(ped, tardar[[]string](time.Hour, nil, nil)), 300*time.Millisecond, time.Second)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("err = %v", err)
		}
	})
}
