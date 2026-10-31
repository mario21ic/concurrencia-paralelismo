package main

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"go.uber.org/goleak"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

func TestElPoolLimitaLasConexionesAbiertas(t *testing.T) {
	ResetDrv(10 * time.Millisecond)
	db := Abrir(3, 3)
	defer db.Close()
	var wg sync.WaitGroup
	for range 30 {
		wg.Go(func() { consultar(context.Background(), db) })
	}
	wg.Wait()
	if a := Drv.Aperturas.Load(); a > 3 {
		t.Fatalf("el driver abrió %d conexiones con MaxOpenConns=3", a)
	}
	if db.Stats().WaitCount == 0 {
		t.Fatal("con 30 goroutines y 3 conexiones debió haber esperas")
	}
}

func TestRendimientoEscalaConElPool(t *testing.T) {
	pequeno := ExpPool(2, 16, 4)
	grande := ExpPool(8, 16, 4)
	t.Logf("pool 2: %.0f/s, pool 8: %.0f/s", pequeno.QPS, grande.QPS)
	if grande.QPS < 2.5*pequeno.QPS {
		t.Fatalf("4× conexiones debería dar ≥2.5× consultas/s (%.0f vs %.0f)", grande.QPS, pequeno.QPS)
	}
}

func TestAgotamientoDelPoolInterbloquea(t *testing.T) {
	if ok, venc := ExpAgotamiento(4, 4, false); ok != 0 || venc != 4 {
		t.Fatalf("pool=goroutines y consulta fuera de la tx debía interbloquear: ok=%d vencidas=%d", ok, venc)
	}
}

func TestAgotamientoSeEvitaConMargenOConTx(t *testing.T) {
	if ok, _ := ExpAgotamiento(4, 8, false); ok != 4 {
		t.Fatalf("con 2 conexiones por goroutine debían terminar todas: ok=%d", ok)
	}
	if ok, _ := ExpAgotamiento(4, 4, true); ok != 4 {
		t.Fatalf("usando la misma tx debían terminar todas: ok=%d", ok)
	}
}

func TestFilasSinCerrarRetienenLaConexion(t *testing.T) {
	n, err := ExpFilasSinCerrar(false, 5)
	if n != 1 || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("sin Close: completadas=%d err=%v", n, err)
	}
	if n, err = ExpFilasSinCerrar(true, 5); n != 5 || err != nil {
		t.Fatalf("con Close: completadas=%d err=%v", n, err)
	}
}

func TestRecorrerTodasLasFilasLiberaLaConexion(t *testing.T) {
	// rows.Next() devuelve false al llegar al final y database/sql cierra las filas solo.
	ResetDrv(time.Millisecond)
	db := Abrir(1, 1)
	defer db.Close()
	for range 3 {
		filas, err := db.Query("SELECT n")
		if err != nil {
			t.Fatal(err)
		}
		for filas.Next() {
		}
	}
	if Drv.RowsAbiertas.Load() != 0 {
		t.Fatalf("filas abiertas: %d", Drv.RowsAbiertas.Load())
	}
}

func TestCancelarElContextoHaceRollbackDeLaTx(t *testing.T) {
	rb, cm, err := ExpCancelarTx()
	if rb != 1 || cm != 0 || !errors.Is(err, sql.ErrTxDone) {
		t.Fatalf("rollbacks=%d commits=%d err=%v", rb, cm, err)
	}
}

func TestCancelarUnaConsultaDevuelveLaConexion(t *testing.T) {
	ResetDrv(time.Second)
	db := Abrir(1, 1)
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := consultar(ctx, db); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err=%v", err)
	}
	Drv.Latencia.Store(int64(time.Millisecond))
	if err := consultar(context.Background(), db); err != nil { // la única conexión volvió al pool
		t.Fatalf("la conexión no se devolvió al pool: %v", err)
	}
	if db.Stats().InUse != 0 {
		t.Fatalf("InUse=%d", db.Stats().InUse)
	}
}

func TestConexionesOciosas(t *testing.T) {
	if a := ExpOciosas(0); a != 100 {
		t.Fatalf("MaxIdleConns=0: aperturas=%d, esperado 100", a)
	}
	if a := ExpOciosas(2); a != 1 {
		t.Fatalf("MaxIdleConns=2: aperturas=%d, esperado 1", a)
	}
}
