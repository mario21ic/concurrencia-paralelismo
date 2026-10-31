//go:build sleep

package lotes

import (
	"sync"
	"testing"
	"time"
)

// La MALA versión: usa un ticker real y Sleep para "esperar a que pase algo".
// Es lenta y frágil: si la máquina va cargada, 120 ms pueden no bastar.
func TestVuelcaPorTiempo_ConSleep(t *testing.T) {
	var mu sync.Mutex
	var lotes [][]int
	tk := time.NewTicker(50 * time.Millisecond)
	defer tk.Stop()
	a := Nuevo(100, tk.C, func(l []int) {
		mu.Lock()
		lotes = append(lotes, l)
		mu.Unlock()
	})
	defer a.Cerrar()

	a.Agregar(1)
	a.Agregar(2)
	time.Sleep(120 * time.Millisecond) // ¿y si el ticker se retrasa?

	mu.Lock()
	defer mu.Unlock()
	if len(lotes) != 1 {
		t.Fatalf("lotes = %v", lotes)
	}
}
