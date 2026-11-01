package main

import (
	"testing"
	"testing/synctest"
	"time"
)

// Con E/S pura el tiempo es ⌈tareas/workers⌉ × 10 ms exactos: el paralelismo solo solapa esperas.
func TestESEscalaConLosWorkers(t *testing.T) {
	for _, w := range []int{1, 4, 16, 64} {
		synctest.Test(t, func(t *testing.T) {
			d := Ejecutar(64, w, EsperaES)
			esperado := time.Duration((64+w-1)/w) * tarea
			if d != esperado {
				t.Errorf("workers=%d: %v, esperado %v", w, d, esperado)
			}
		})
	}
}

func TestHashEsDeterminista(t *testing.T) {
	if Hash(1000) != Hash(1000) || Hash(1000) == Hash(1001) {
		t.Fatal("Hash debe ser determinista y sensible al nº de rondas")
	}
	if rondasCPU < 1000 {
		t.Fatalf("calibración absurda: %d rondas", rondasCPU)
	}
}
