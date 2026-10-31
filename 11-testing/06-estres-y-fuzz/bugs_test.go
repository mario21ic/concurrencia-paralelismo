//go:build bugs

package estres

import "testing"

// anilloMalo da la vuelta mal: tras llegar al final, vuelve a 1 en vez de a 0.
// El bug SOLO se manifiesta después de varias vueltas completas del buffer: un
// test escrito a mano con pocos elementos lo pasaría sin problema.
type anilloMalo struct {
	buf   []byte
	head  int
	count int
}

func nuevoMalo(cap int) *anilloMalo { return &anilloMalo{buf: make([]byte, cap)} }

func (a *anilloMalo) Push(v byte) bool {
	if a.count == len(a.buf) {
		return false
	}
	a.buf[(a.head+a.count)%len(a.buf)] = v
	a.count++
	return true
}

func (a *anilloMalo) Pop() (byte, bool) {
	if a.count == 0 {
		return 0, false
	}
	v := a.buf[a.head]
	a.head++
	if a.head == len(a.buf) {
		a.head = 1 // BUG: debería ser 0
	}
	a.count--
	return v, true
}

func (a *anilloMalo) Len() int { return a.count }

func FuzzAnilloMalo(f *testing.F) {
	f.Add([]byte{0, 1}) // semilla trivial: el fuzzer debe encontrar el resto solo
	f.Fuzz(func(t *testing.T, ops []byte) {
		comprobarContraModelo(t, nuevoMalo(4), 4, ops)
	})
}

// anilloRacy: Len sin tomar el lock (data race).
type anilloRacy struct{ *Anillo }

func (a anilloRacy) Len() int { return a.count } // ¡sin mu.Lock()!

// Este test PASA la comprobación de rango casi siempre (el valor leído "parece"
// razonable); solo el detector de carreras (-race) revela el defecto.
func TestLenSinLock(t *testing.T) {
	probarLenConcurrente(t, anilloRacy{NuevoAnillo(8)})
}
