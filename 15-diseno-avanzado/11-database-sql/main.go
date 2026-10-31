// Ejercicio 11: database/sql — pool, transacciones y cancelación (con un driver simulado).
//
//	go run .
package main

import (
	"database/sql"
	"errors"
	"fmt"
)

func main() {
	fmt.Println("== A) tamaño del pool: 64 goroutines × 10 consultas de 20 ms ==")
	fmt.Printf("  %-12s %10s %18s %18s\n", "MaxOpenConns", "consultas/s", "veces que esperó", "espera media")
	for _, n := range []int{1, 2, 4, 8, 16, 32, 64} {
		r := ExpPool(n, 64, 10)
		fmt.Printf("  %-12d %10.0f %18d %18v\n", n, r.QPS, r.Esperas, r.EsperaMedia.Round(1e6))
	}
	fmt.Println("  Techo = MaxOpenConns / latencia (n/20 ms). Más conexiones que las que la BD aguanta tampoco es gratis (ver README).")

	fmt.Println("\n== B) interbloqueo por agotamiento del pool: N goroutines con una transacción abierta + una consulta dentro ==")
	for _, c := range []struct {
		g, pool int
		tx      bool
		d       string
	}{
		{4, 4, false, "consulta con db (pide 2.ª conexión), pool = 4"},
		{4, 5, false, "consulta con db, pool = 5 (1 de margen)"},
		{4, 8, false, "consulta con db, pool = 8 (2 por goroutine)"},
		{4, 4, true, "consulta con tx (misma conexión), pool = 4"},
	} {
		ok, venc := ExpAgotamiento(c.g, c.pool, c.tx)
		fmt.Printf("  %-52s → terminadas: %d, vencieron el plazo: %d\n", c.d, ok, venc)
	}

	fmt.Println("\n== C) filas sin cerrar con un pool de 1 conexión (5 consultas seguidas) ==")
	n1, e1 := ExpFilasSinCerrar(false, 5)
	n2, e2 := ExpFilasSinCerrar(true, 5)
	fmt.Printf("  sin rows.Close(): %d completadas, error: %v\n  con rows.Close(): %d completadas, error: %v\n", n1, e1, n2, e2)

	fmt.Println("\n== D) cancelar el contexto de una transacción ==")
	rb, cm, err := ExpCancelarTx()
	fmt.Printf("  ROLLBACK recibidos por el driver: %d, COMMIT: %d; tx.Commit() → %v (¿es sql.ErrTxDone? %v)\n",
		rb, cm, err, errors.Is(err, sql.ErrTxDone))

	fmt.Println("\n== E) conexiones ociosas: 100 consultas seguidas con MaxOpenConns=8 ==")
	for _, o := range []int{0, 1, 2, 8} {
		fmt.Printf("  MaxIdleConns=%d → el driver abrió %d conexiones\n", o, ExpOciosas(o))
	}
}
