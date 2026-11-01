package main

import "testing"

func TestVersionesSeguras(t *testing.T) {
	esperado := goroutines * cadaUna
	if got := ContarMutex(); got != esperado {
		t.Errorf("mutex = %d, esperado %d", got, esperado)
	}
	if got := ContarAtomic(); got != esperado {
		t.Errorf("atomic = %d, esperado %d", got, esperado)
	}
	// ContarMal no se prueba aquí: con -race el detector fallaría el test (justo lo que queremos que
	// el estudiante vea con `go run -race . -mal`).
}
