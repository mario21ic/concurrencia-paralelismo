package traza

import (
	"bytes"
	"context"
	"runtime/trace"
	"sync"
	"testing"
	"time"
)

// generar produce una traza real con una tarea, regiones, un log y una espera conocida.
func generar(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := trace.Start(&buf); err != nil {
		t.Skipf("no se pudo iniciar la traza (¿otro consumidor activo?): %v", err)
	}
	ctx, tarea := trace.NewTask(context.Background(), "mi-tarea")
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			trace.WithRegion(ctx, "dormir", func() { time.Sleep(10 * time.Millisecond) })
		})
	}
	trace.Log(ctx, "cat", "hola")
	wg.Wait()
	tarea.End()
	trace.Stop()
	return buf.Bytes()
}

func TestAnalizar(t *testing.T) {
	res, err := Analizar(bytes.NewReader(generar(t)))
	if err != nil {
		t.Fatal(err)
	}
	if r := res.Regiones["dormir"]; r == nil || r.N != 4 || r.Total < 4*8*time.Millisecond {
		t.Fatalf("región dormir = %+v", r)
	}
	if k := res.Tareas["mi-tarea"]; k == nil || k.N != 1 {
		t.Fatalf("tarea = %+v", k)
	}
	if res.Logs != 1 {
		t.Fatalf("logs = %d, want 1", res.Logs)
	}
	if res.PorMotivo["sleep"] < 4*8*time.Millisecond {
		t.Fatalf("espera por sleep = %v; se esperaban >= 32ms", res.PorMotivo["sleep"])
	}
	if res.Eventos == 0 || res.Duracion <= 0 {
		t.Fatalf("resumen vacío: %+v", res)
	}
}

func TestTrazaInvalida(t *testing.T) {
	if _, err := Analizar(bytes.NewReader([]byte("esto no es una traza"))); err == nil {
		t.Fatal("se esperaba error con una traza inválida")
	}
}
