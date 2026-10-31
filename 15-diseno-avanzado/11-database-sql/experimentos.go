package main

import (
	"context"
	"database/sql"
	"sync"
	"sync/atomic"
	"time"
)

// consultar ejecuta una consulta y consume todas las filas, cerrándolas (el patrón correcto).
func consultar(ctx context.Context, q interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}) error {
	filas, err := q.QueryContext(ctx, "SELECT n")
	if err != nil {
		return err
	}
	defer filas.Close() // SIEMPRE: devuelve la conexión al pool
	for filas.Next() {
		var n int
		if err := filas.Scan(&n); err != nil {
			return err
		}
	}
	return filas.Err()
}

// ---- A) Tamaño del pool: el rendimiento está limitado por MaxOpenConns / latencia ----

type ResultadoPool struct {
	QPS         float64
	Esperas     int64
	EsperaMedia time.Duration
}

func ExpPool(maxAbiertas, goroutines, consultasCada int) ResultadoPool {
	ResetDrv(20 * time.Millisecond)
	db := Abrir(maxAbiertas, maxAbiertas)
	defer db.Close()
	var wg sync.WaitGroup
	t0 := time.Now()
	for range goroutines {
		wg.Go(func() {
			for range consultasCada {
				consultar(context.Background(), db)
			}
		})
	}
	wg.Wait()
	s := db.Stats()
	r := ResultadoPool{QPS: float64(goroutines*consultasCada) / time.Since(t0).Seconds(), Esperas: s.WaitCount}
	if s.WaitCount > 0 {
		r.EsperaMedia = s.WaitDuration / time.Duration(s.WaitCount)
	}
	return r
}

// ---- B) Interbloqueo por agotamiento del pool ----

// ExpAgotamiento: `goroutines` goroutines abren una transacción (retienen 1 conexión cada una) y,
// DENTRO de ella, hacen otra consulta. Si usan `db` (no `tx`) necesitan una SEGUNDA conexión; con
// un pool de tamaño `goroutines` todas retienen una y esperan otra: nadie la libera. Devuelve
// cuántas terminaron y cuántas vencieron el plazo.
func ExpAgotamiento(goroutines, maxAbiertas int, usarTx bool) (ok, vencidas int) {
	ResetDrv(5 * time.Millisecond)
	db := Abrir(maxAbiertas, maxAbiertas)
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
	defer cancel()
	var nOK, nVenc atomic.Int64
	var wg sync.WaitGroup
	barrera := make(chan struct{})
	var llegadas atomic.Int64
	for range goroutines {
		wg.Go(func() {
			t, err := db.BeginTx(ctx, nil)
			if err != nil {
				nVenc.Add(1)
				return
			}
			defer t.Rollback()
			if llegadas.Add(1) == int64(goroutines) {
				close(barrera) // todas ya retienen su conexión
			}
			<-barrera
			var err2 error
			if usarTx {
				err2 = consultar(ctx, t) // misma conexión: no necesita otra
			} else {
				err2 = consultar(ctx, db) // OTRA conexión del pool: puede no haber
			}
			if err2 != nil {
				nVenc.Add(1)
				return
			}
			t.Commit()
			nOK.Add(1)
		})
	}
	wg.Wait()
	return int(nOK.Load()), int(nVenc.Load())
}

// ---- C) Filas sin cerrar ----

// ExpFilasSinCerrar hace `n` consultas secuenciales con un pool de UNA conexión. Si no cierra las
// filas (y no las recorre hasta el final), la conexión sigue ocupada y la siguiente consulta se bloquea.
func ExpFilasSinCerrar(cerrar bool, n int) (completadas int, err error) {
	ResetDrv(time.Millisecond)
	db := Abrir(1, 1)
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	for range n {
		filas, e := db.QueryContext(ctx, "SELECT n")
		if e != nil {
			return completadas, e
		}
		filas.Next() // lee solo la primera fila y se "olvida" del resto
		if cerrar {
			filas.Close()
		}
		completadas++
	}
	return
}

// ---- D) Cancelación de una transacción ----

// ExpCancelarTx abre una transacción con un contexto, lo cancela y observa: el driver recibe
// ROLLBACK automáticamente y Commit devuelve un error.
func ExpCancelarTx() (rollbacks, commits int64, errCommit error) {
	ResetDrv(time.Millisecond)
	db := Abrir(2, 2)
	defer db.Close()
	ctx, cancel := context.WithCancel(context.Background())
	tx, _ := db.BeginTx(ctx, nil)
	tx.ExecContext(ctx, "UPDATE ...")
	cancel()
	time.Sleep(50 * time.Millisecond) // database/sql vigila ctx en una goroutine propia
	errCommit = tx.Commit()
	return Drv.Rollbacks.Load(), Drv.Commits.Load(), errCommit
}

// ---- E) Conexiones ociosas: reutilizar frente a abrir una por consulta ----

func ExpOciosas(maxOciosas int) (aperturas int64) {
	ResetDrv(time.Millisecond)
	db := Abrir(8, maxOciosas)
	defer db.Close()
	for range 100 { // 100 consultas seguidas
		consultar(context.Background(), db)
	}
	return Drv.Aperturas.Load()
}
