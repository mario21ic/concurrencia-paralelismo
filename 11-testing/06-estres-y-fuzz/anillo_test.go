package estres

import (
	"runtime"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
)

// cola es lo que comparten las implementaciones correcta y defectuosas.
type cola interface {
	Push(byte) bool
	Pop() (byte, bool)
	Len() int
}

// modelo: la especificación más simple posible (un slice).
type modelo struct {
	cap  int
	elem []byte
}

func (m *modelo) Push(v byte) bool {
	if len(m.elem) == m.cap {
		return false
	}
	m.elem = append(m.elem, v)
	return true
}
func (m *modelo) Pop() (byte, bool) {
	if len(m.elem) == 0 {
		return 0, false
	}
	v := m.elem[0]
	m.elem = m.elem[1:]
	return v, true
}
func (m *modelo) Len() int { return len(m.elem) }

// comprobarContraModelo interpreta cada byte como una operación:
// par => Push(byte), impar => Pop. Compara la cola real con el modelo en cada paso.
func comprobarContraModelo(t *testing.T, real cola, cap int, ops []byte) {
	t.Helper()
	m := &modelo{cap: cap}
	for i, b := range ops {
		if b%2 == 0 {
			if got, want := real.Push(b), m.Push(b); got != want {
				t.Fatalf("op %d Push(%d) = %v, modelo %v", i, b, got, want)
			}
		} else {
			gv, gok := real.Pop()
			wv, wok := m.Pop()
			if gv != wv || gok != wok {
				t.Fatalf("op %d Pop() = (%d,%v), modelo (%d,%v)", i, gv, gok, wv, wok)
			}
		}
		if real.Len() != m.Len() {
			t.Fatalf("op %d Len() = %d, modelo %d", i, real.Len(), m.Len())
		}
	}
}

// ---- Fuzzing: con `go test` ejecuta solo las semillas; con -fuzz explora ----

func FuzzAnillo(f *testing.F) {
	f.Add([]byte{0, 2, 4, 1, 1, 1})                   // llena y vacía
	f.Add([]byte{0, 2, 4, 6, 8, 1, 1, 1, 1, 1, 0, 1}) // da la vuelta
	f.Add([]byte{1, 1, 1})                            // pops sobre vacía
	f.Fuzz(func(t *testing.T, ops []byte) {
		comprobarContraModelo(t, NuevoAnillo(4), 4, ops)
	})
}

// ---- Estrés: muchos productores y consumidores; invariante de conservación ----

func TestEstresConservacion(t *testing.T) {
	const (
		productores  = 8
		consumidores = 8
		porProd      = 2000
	)
	q := NuevoAnillo(16)
	total := productores * porProd

	// Cada elemento se identifica por (productor, número); como la cola guarda
	// bytes, se envía el número módulo 251 y se lleva la cuenta por productor
	// en el lado productor/consumidor mediante contadores globales.
	var enviados, recibidos, sumaEnv, sumaRec atomic.Int64
	var wg sync.WaitGroup

	for p := range productores {
		wg.Go(func() {
			for i := range porProd {
				v := byte((p*31 + i) % 251)
				for !q.Push(v) { // cola llena: reintentar CEDIENDO el turno
					runtime.Gosched() // sin esto, con -cpu 1 cada reintento consume una franja de 10 ms
				}
				enviados.Add(1)
				sumaEnv.Add(int64(v))
			}
		})
	}
	for range consumidores {
		wg.Go(func() {
			for recibidos.Load() < int64(total) {
				if v, ok := q.Pop(); ok {
					recibidos.Add(1)
					sumaRec.Add(int64(v))
				} else {
					runtime.Gosched()
				}
			}
		})
	}
	wg.Wait()

	if enviados.Load() != int64(total) || recibidos.Load() != int64(total) {
		t.Fatalf("enviados=%d recibidos=%d, want %d", enviados.Load(), recibidos.Load(), total)
	}
	if sumaEnv.Load() != sumaRec.Load() {
		t.Fatalf("suma enviada %d != suma recibida %d (se perdieron o duplicaron valores)", sumaEnv.Load(), sumaRec.Load())
	}
	if q.Len() != 0 {
		t.Fatalf("Len = %d, want 0", q.Len())
	}
}

// Un solo productor y un solo consumidor: aquí SÍ se puede afirmar el orden FIFO.
func TestEstresOrdenFIFOUnoAUno(t *testing.T) {
	q := NuevoAnillo(8)
	const n = 20000
	var wg sync.WaitGroup
	wg.Go(func() {
		for i := range n {
			for !q.Push(byte(i % 251)) {
				runtime.Gosched()
			}
		}
	})
	got := make([]byte, 0, n)
	wg.Go(func() {
		for len(got) < n {
			if v, ok := q.Pop(); ok {
				got = append(got, v)
			} else {
				runtime.Gosched()
			}
		}
	})
	wg.Wait()
	for i, v := range got {
		if v != byte(i%251) {
			t.Fatalf("got[%d] = %d, want %d: se rompió el orden FIFO", i, v, i%251)
		}
	}
}

// Len concurrente con Push/Pop: con -race detecta si Len no toma el lock.
func TestLenConcurrente(t *testing.T) {
	probarLenConcurrente(t, NuevoAnillo(8))
}

func probarLenConcurrente(t *testing.T, q cola) {
	t.Helper()
	var wg sync.WaitGroup
	parar := make(chan struct{})
	wg.Go(func() {
		for i := 0; ; i++ {
			select {
			case <-parar:
				return
			default:
				q.Push(byte(i))
				q.Pop()
			}
		}
	})
	for range 5000 {
		if n := q.Len(); n < 0 || n > 8 {
			t.Errorf("Len fuera de rango: %d", n)
		}
	}
	close(parar)
	wg.Wait()
}

func TestSecuencialSimple(t *testing.T) {
	q := NuevoAnillo(3)
	for _, v := range []byte{1, 2, 3} {
		if !q.Push(v) {
			t.Fatal("Push debía caber")
		}
	}
	if q.Push(4) {
		t.Fatal("Push en cola llena debía fallar")
	}
	var got []byte
	for range 3 {
		v, _ := q.Pop()
		got = append(got, v)
	}
	if !slices.Equal(got, []byte{1, 2, 3}) {
		t.Fatalf("got %v", got)
	}
}
