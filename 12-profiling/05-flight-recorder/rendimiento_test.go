package main

import (
	"runtime/trace"
	"sync"
	"testing"
	"time"
)

func carga(b *testing.B) {
	var mu sync.Mutex
	n := 0
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mu.Lock()
			n++
			mu.Unlock()
			ch := make(chan struct{})
			go func() { close(ch) }() // genera eventos de planificador
			<-ch
		}
	})
}

func BenchmarkSinRecorder(b *testing.B) { carga(b) }

func BenchmarkConRecorder(b *testing.B) {
	fr := trace.NewFlightRecorder(trace.FlightRecorderConfig{MinAge: time.Second, MaxBytes: 4 << 20})
	if err := fr.Start(); err != nil {
		b.Fatal(err)
	}
	defer fr.Stop()
	carga(b)
}

func TestElRecorderSeActivaYDesactiva(t *testing.T) {
	fr := trace.NewFlightRecorder(trace.FlightRecorderConfig{})
	if fr.Enabled() {
		t.Fatal("no debería estar activo antes de Start")
	}
	if err := fr.Start(); err != nil {
		t.Fatal(err)
	}
	if !fr.Enabled() {
		t.Fatal("debería estar activo tras Start")
	}
	fr.Stop()
	if fr.Enabled() {
		t.Fatal("no debería estar activo tras Stop")
	}
}
