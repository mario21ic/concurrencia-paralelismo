package main

import (
	"sync"
	"sync/atomic"
)

// PilaIdx es una pila de Treiber sobre un ARRAY de nodos reutilizables (como en un kernel, un
// pool de memoria sin GC o una estructura en memoria compartida). Los "punteros" son ÍNDICES, y un
// índice liberado puede volver a usarse: ahí aparece el ABA.
//
// cabeza empaqueta (etiqueta << 32 | índice+1); 0 = vacía. Con conEtiqueta, cada modificación
// incrementa la etiqueta, así que "el mismo índice" con otra etiqueta NO se confunde.
type PilaIdx struct {
	conEtiqueta bool
	nodos       []struct{ val, sig atomic.Int64 }
	cabeza      atomic.Uint64
	mu          sync.Mutex // protege la lista de libres (no es lo que estudiamos)
	libres      []int
}

func NuevaPilaIdx(n int, conEtiqueta bool) *PilaIdx {
	p := &PilaIdx{conEtiqueta: conEtiqueta, nodos: make([]struct{ val, sig atomic.Int64 }, n)}
	for i := n - 1; i >= 0; i-- {
		p.libres = append(p.libres, i)
	}
	return p
}

func empaquetar(etq uint32, idx int) uint64 { return uint64(etq)<<32 | uint64(idx+1) }
func desempaquetar(h uint64) (etq uint32, idx int) {
	return uint32(h >> 32), int(uint32(h)) - 1
}

func (p *PilaIdx) siguienteEtiqueta(etq uint32) uint32 {
	if p.conEtiqueta {
		return etq + 1
	}
	return 0
}

func (p *PilaIdx) tomarLibre() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	i := p.libres[len(p.libres)-1]
	p.libres = p.libres[:len(p.libres)-1]
	return i
}
func (p *PilaIdx) devolver(i int) { p.mu.Lock(); p.libres = append(p.libres, i); p.mu.Unlock() }

func (p *PilaIdx) Push(v int64) { p.pushEn(p.tomarLibre(), v) }

// PushEn inserta usando un hueco concreto (lo que haría un asignador que recicla el hueco recién liberado).
func (p *PilaIdx) PushEn(hueco int, v int64) {
	p.mu.Lock()
	for j, l := range p.libres {
		if l == hueco {
			p.libres = append(p.libres[:j], p.libres[j+1:]...)
			break
		}
	}
	p.mu.Unlock()
	p.pushEn(hueco, v)
}

func (p *PilaIdx) pushEn(i int, v int64) {
	p.nodos[i].val.Store(v)
	for {
		h := p.cabeza.Load()
		etq, viejo := desempaquetar(h)
		p.nodos[i].sig.Store(int64(viejo))
		if p.cabeza.CompareAndSwap(h, empaquetar(p.siguienteEtiqueta(etq), i)) {
			return
		}
	}
}

// Instantanea es la primera mitad de Pop: lo que "ve" una goroutine antes de ser interrumpida.
type Instantanea struct {
	h   uint64
	idx int
	sig int
}

func (p *PilaIdx) Leer() (Instantanea, bool) {
	h := p.cabeza.Load()
	_, idx := desempaquetar(h)
	if idx < 0 {
		return Instantanea{}, false
	}
	return Instantanea{h, idx, int(p.nodos[idx].sig.Load())}, true
}

// Confirmar es la segunda mitad de Pop: el CAS que "solo comprueba que la cabeza sigue igual".
func (p *PilaIdx) Confirmar(s Instantanea) (int64, bool) {
	etq, _ := desempaquetar(s.h)
	if !p.cabeza.CompareAndSwap(s.h, empaquetar(p.siguienteEtiqueta(etq), s.sig)) {
		return 0, false
	}
	v := p.nodos[s.idx].val.Load()
	p.devolver(s.idx)
	return v, true
}

func (p *PilaIdx) Pop() (int64, bool) {
	for {
		s, ok := p.Leer()
		if !ok {
			return 0, false
		}
		if v, ok := p.Confirmar(s); ok {
			return v, true
		}
	}
}

// Contenido recorre la pila (para mostrarla); se detiene si detecta un ciclo.
func (p *PilaIdx) Contenido() string {
	s := "["
	_, idx := desempaquetar(p.cabeza.Load())
	for n := 0; idx >= 0 && n < len(p.nodos)+1; n++ {
		if n > 0 {
			s += " → "
		}
		s += string(rune(p.nodos[idx].val.Load()))
		idx = int(p.nodos[idx].sig.Load())
	}
	return s + "]"
}

// EscenarioABA ejecuta, de forma DETERMINISTA, el entrelazado clásico (A, B, C son valores; cada
// uno vive en un hueco del array):
//
//	pila = A → B → C
//	T1: lee cabeza=A, siguiente=B   ── (la interrumpen antes del CAS)
//	T2: Pop() → A ; Pop() → B ; Push(A') en el MISMO hueco que tenía A      (pila: A' → C)
//	T1: CAS(cabeza: A → B)           ¿tiene éxito? La cabeza "parece" la misma…
//
// Sin etiqueta el CAS triunfa y la pila queda apuntando a B, un nodo YA extraído (resucitado);
// A' se pierde. Con etiqueta el CAS falla y T1 reintenta con datos frescos.
func EscenarioABA(conEtiqueta bool) (casTriunfa bool, antes, despues string) {
	p := NuevaPilaIdx(4, conEtiqueta)
	p.PushEn(2, 'C')
	p.PushEn(1, 'B')
	p.PushEn(0, 'A')
	antes = p.Contenido()
	t1, _ := p.Leer() // T1: cabeza=A (hueco 0), siguiente=B (hueco 1)

	p.Pop()          // T2 saca A y libera el hueco 0
	p.Pop()          // T2 saca B y libera el hueco 1
	p.PushEn(0, 'X') // T2 mete A' (aquí 'X') en el hueco 0: la cabeza vuelve a ser "el hueco 0"
	_, casTriunfa = p.Confirmar(t1)
	return casTriunfa, antes, p.Contenido()
}
