package main

import (
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConcurrenteIgualASecuencial(t *testing.T) {
	archivos, err := listar("textos")
	if err != nil || len(archivos) == 0 {
		t.Fatal("faltan archivos de prueba")
	}
	esperado, err := contarSecuencial(archivos, 4)
	if err != nil {
		t.Fatal(err)
	}
	for _, workers := range []int{1, 2, 8} {
		got, errs := contarConcurrente(archivos, workers, 4)
		if len(errs) > 0 {
			t.Fatalf("workers=%d errores: %v", workers, errs)
		}
		if !maps.Equal(got, esperado) {
			t.Errorf("workers=%d: resultado distinto del secuencial", workers)
		}
	}
}

func TestArchivoInexistenteNoBloquea(t *testing.T) {
	dir := t.TempDir()
	ok := filepath.Join(dir, "ok.txt")
	if err := os.WriteFile(ok, []byte("hola hola mundo"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, errs := contarConcurrente([]string{ok, filepath.Join(dir, "no-existe.txt")}, 2, 4)
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "no-existe") {
		t.Errorf("se esperaba 1 error, got %v", errs)
	}
	if got["hola"] != 2 {
		t.Errorf("hola = %d, want 2", got["hola"])
	}
}

func TestTopDesempate(t *testing.T) {
	got := top(map[string]int{"b": 2, "a": 2, "c": 5}, 2)
	if got[0].palabra != "c" || got[1].palabra != "a" {
		t.Errorf("orden inesperado: %v", got)
	}
}
