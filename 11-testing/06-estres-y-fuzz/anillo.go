// Package estres: una cola FIFO acotada (buffer circular) segura para uso
// concurrente. Sirve para practicar tres técnicas complementarias:
//
//   - fuzzing contra un modelo secuencial  -> bugs de LÓGICA (p. ej. al dar la vuelta)
//   - pruebas de estrés con -race/-cpu     -> bugs de SINCRONIZACIÓN
//   - invariantes (conservación)           -> pérdida/duplicado de elementos
package estres

import "sync"

type Anillo struct {
	mu    sync.Mutex
	buf   []byte
	head  int // índice del siguiente a sacar
	count int
}

func NuevoAnillo(capacidad int) *Anillo { return &Anillo{buf: make([]byte, capacidad)} }

// Push añade v; false si está llena.
func (a *Anillo) Push(v byte) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.count == len(a.buf) {
		return false
	}
	a.buf[(a.head+a.count)%len(a.buf)] = v
	a.count++
	return true
}

// Pop saca el más antiguo; ok=false si está vacía.
func (a *Anillo) Pop() (v byte, ok bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.count == 0 {
		return 0, false
	}
	v = a.buf[a.head]
	a.head = (a.head + 1) % len(a.buf)
	a.count--
	return v, true
}

func (a *Anillo) Len() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.count
}
