package main

import (
	"context"
	"errors"
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func generar(n int) ([]string, int) {
	ls := make([]string, n)
	s := 0
	for i := range ls {
		ls[i] = strconv.Itoa(i)
		s += i
	}
	return ls, s
}

func TestSumaCorrecta(t *testing.T) {
	ls, esperado := generar(5000)
	for _, w := range []int{1, 3, 16} {
		got, err := SumarLineas(context.Background(), ls, w, nil)
		if err != nil || got != esperado {
			t.Fatalf("workers=%d got=%d err=%v want=%d", w, got, err, esperado)
		}
	}
}

func TestErrorDetieneTodo(t *testing.T) {
	ls, _ := generar(100_000)
	ls[10] = "malo"
	var proc atomic.Int64
	_, err := SumarLineas(context.Background(), ls, 4, &proc)
	if err == nil || !strings.Contains(err.Error(), "malo") {
		t.Fatalf("err=%v", err)
	}
	if proc.Load() > 50_000 {
		t.Fatalf("procesó %d líneas; debería abortar pronto", proc.Load())
	}
}

func TestSinFugas(t *testing.T) {
	base := runtime.NumGoroutine()
	ls, _ := generar(10_000)
	ls[5000] = "x"
	for range 20 {
		SumarLineas(context.Background(), ls, 8, nil)
	}
	time.Sleep(30 * time.Millisecond)
	if d := runtime.NumGoroutine(); d > base+1 {
		t.Fatalf("fuga: base=%d después=%d", base, d)
	}
}

func TestContextoCancelado(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ls, _ := generar(1000)
	if _, err := SumarLineas(ctx, ls, 4, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("err=%v, want Canceled", err)
	}
}
