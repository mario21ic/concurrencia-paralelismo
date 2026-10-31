// Package main (ejercicio 11): concurrencia con database/sql — pool de conexiones, transacciones y
// cancelación. Usa un driver SIMULADO (sin base de datos real) que cuenta aperturas, cierres,
// commits y rollbacks, para poder observar cómo se comporta el pool.
package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"sync/atomic"
	"time"
)

// Contadores globales del driver simulado.
var Drv struct {
	Aperturas, Cierres, Commits, Rollbacks atomic.Int64
	RowsAbiertas                           atomic.Int64
	Latencia                               atomic.Int64 // duración de cada consulta en ns
}

func ResetDrv(latencia time.Duration) {
	Drv.Aperturas.Store(0)
	Drv.Cierres.Store(0)
	Drv.Commits.Store(0)
	Drv.Rollbacks.Store(0)
	Drv.RowsAbiertas.Store(0)
	Drv.Latencia.Store(int64(latencia))
}

func init() { sql.Register("fakedb", fakeDriver{}) }

type fakeDriver struct{}

func (fakeDriver) Open(string) (driver.Conn, error) {
	Drv.Aperturas.Add(1)
	time.Sleep(time.Millisecond) // abrir una conexión real cuesta (TCP + autenticación)
	return &conn{}, nil
}

type conn struct{}

func (*conn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("fakedb: solo QueryContext/ExecContext")
}
func (*conn) Close() error { Drv.Cierres.Add(1); return nil }
func (c *conn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (*conn) BeginTx(ctx context.Context, _ driver.TxOptions) (driver.Tx, error) {
	return tx{}, ctx.Err()
}

func dormir(ctx context.Context) error {
	t := time.NewTimer(time.Duration(Drv.Latencia.Load()))
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err() // una consulta de verdad se cancelaría enviando un "cancel" al servidor
	}
}

func (*conn) ExecContext(ctx context.Context, _ string, _ []driver.NamedValue) (driver.Result, error) {
	if err := dormir(ctx); err != nil {
		return nil, err
	}
	return driver.RowsAffected(1), nil
}

func (*conn) QueryContext(ctx context.Context, _ string, _ []driver.NamedValue) (driver.Rows, error) {
	if err := dormir(ctx); err != nil {
		return nil, err
	}
	Drv.RowsAbiertas.Add(1)
	return &rows{n: 3}, nil
}

type tx struct{}

func (tx) Commit() error   { Drv.Commits.Add(1); return nil }
func (tx) Rollback() error { Drv.Rollbacks.Add(1); return nil }

type rows struct{ i, n int }

func (*rows) Columns() []string { return []string{"n"} }
func (*rows) Close() error      { Drv.RowsAbiertas.Add(-1); return nil }
func (r *rows) Next(dest []driver.Value) error {
	if r.i >= r.n {
		return io.EOF
	}
	dest[0] = int64(r.i)
	r.i++
	return nil
}

func Abrir(maxAbiertas, maxOciosas int) *sql.DB {
	db, _ := sql.Open("fakedb", "")
	db.SetMaxOpenConns(maxAbiertas)
	db.SetMaxIdleConns(maxOciosas)
	return db
}
