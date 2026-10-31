package main

import (
	"cmp"
	"iter"
	"math/rand/v2"
	"runtime"
	"sync"
	"sync/atomic"
)

const maxNivel = 16

// nodo de la lista. `marcado` = borrado lógico; `enlazado` = visible en TODOS sus niveles.
type nodo[K cmp.Ordered] struct {
	clave     K
	centinela int8 // -1 cabeza (menor que todo), +1 cola (mayor que todo), 0 nodo normal
	alto      int  // índice del nivel más alto en el que está
	sig       [maxNivel]atomic.Pointer[nodo[K]]
	marcado   atomic.Bool
	enlazado  atomic.Bool
	mu        sync.Mutex
}

// menor dice si n < k (la cabeza es menor que cualquier clave; la cola, mayor).
func (n *nodo[K]) menor(k K) bool {
	switch n.centinela {
	case -1:
		return true
	case 1:
		return false
	}
	return n.clave < k
}

func (n *nodo[K]) es(k K) bool { return n.centinela == 0 && n.clave == k }

// Lista es un conjunto ordenado concurrente: una skip list "perezosa" (Herlihy, Lev, Luchangco,
// Shavit, 2007).
//
//   - Contains es SIN ESPERAS: no toma ningún cerrojo.
//   - Add y Remove solo cierran los pocos nodos que tocan (los predecesores y el nodo borrado).
//   - Borrar es en dos fases: primero se MARCA (borrado lógico) y luego se desenlaza.
//   - El recolector de basura resuelve la reclamación de memoria (en C++ haría falta hazard
//     pointers o épocas) y evita el problema ABA con punteros.
type Lista[K cmp.Ordered] struct {
	cabeza, cola *nodo[K]
}

func Nueva[K cmp.Ordered]() *Lista[K] {
	l := &Lista[K]{cabeza: &nodo[K]{centinela: -1, alto: maxNivel - 1}, cola: &nodo[K]{centinela: 1, alto: maxNivel - 1}}
	for i := range maxNivel {
		l.cabeza.sig[i].Store(l.cola)
	}
	l.cabeza.enlazado.Store(true)
	l.cola.enlazado.Store(true)
	return l
}

// nivelAleatorio: geometría p = 1/2 (nivel 0 con prob. 1/2, nivel 1 con 1/4…).
func nivelAleatorio() int {
	n := 0
	for n < maxNivel-1 && rand.Uint32()&1 == 0 {
		n++
	}
	return n
}

// buscar rellena preds/succs de cada nivel y devuelve el nivel más alto donde está la clave (-1 si no está).
func (l *Lista[K]) buscar(k K, preds, succs *[maxNivel]*nodo[K]) (encontrado int) {
	encontrado = -1
	pred := l.cabeza
	for nivel := maxNivel - 1; nivel >= 0; nivel-- {
		curr := pred.sig[nivel].Load()
		for curr.menor(k) {
			pred = curr
			curr = pred.sig[nivel].Load()
		}
		if encontrado == -1 && curr.es(k) {
			encontrado = nivel
		}
		preds[nivel], succs[nivel] = pred, curr
	}
	return
}

// soltar libera los cerrojos de los predecesores (cada predecesor distinto una sola vez).
func soltar[K cmp.Ordered](preds *[maxNivel]*nodo[K], masAlto int) {
	for n := 0; n <= masAlto; n++ {
		if n == 0 || preds[n] != preds[n-1] {
			preds[n].mu.Unlock()
		}
	}
}

// Contains: sin cerrojos, sin esperas.
func (l *Lista[K]) Contains(k K) bool {
	var preds, succs [maxNivel]*nodo[K]
	n := l.buscar(k, &preds, &succs)
	return n != -1 && succs[n].enlazado.Load() && !succs[n].marcado.Load()
}

// Add inserta k; false si ya estaba.
func (l *Lista[K]) Add(k K) bool {
	alto := nivelAleatorio()
	var preds, succs [maxNivel]*nodo[K]
	for {
		if n := l.buscar(k, &preds, &succs); n != -1 {
			enc := succs[n]
			if !enc.marcado.Load() {
				for !enc.enlazado.Load() { // otro Add lo está terminando: espera a que sea visible
					runtime.Gosched()
				}
				return false
			}
			continue // se está borrando: reintenta
		}
		masAlto, valido := -1, true
		var prev *nodo[K]
		for nivel := 0; valido && nivel <= alto; nivel++ {
			pred, succ := preds[nivel], succs[nivel]
			if pred != prev {
				pred.mu.Lock()
				masAlto, prev = nivel, pred
			}
			valido = !pred.marcado.Load() && !succ.marcado.Load() && pred.sig[nivel].Load() == succ
		}
		if !valido { // alguien cambió el vecindario entre buscar y cerrar
			soltar(&preds, masAlto)
			continue
		}
		nuevo := &nodo[K]{clave: k, alto: alto}
		for nivel := 0; nivel <= alto; nivel++ {
			nuevo.sig[nivel].Store(succs[nivel])
		}
		for nivel := 0; nivel <= alto; nivel++ {
			preds[nivel].sig[nivel].Store(nuevo) // punto de publicación (de abajo arriba)
		}
		nuevo.enlazado.Store(true) // linealización de Add
		soltar(&preds, masAlto)
		return true
	}
}

// Remove elimina k; false si no estaba.
func (l *Lista[K]) Remove(k K) bool {
	var victima *nodo[K]
	marcada := false
	alto := -1
	var preds, succs [maxNivel]*nodo[K]
	for {
		n := l.buscar(k, &preds, &succs)
		if marcada || (n != -1 && puedeBorrarse(succs[n], n)) {
			if !marcada {
				victima = succs[n]
				alto = victima.alto
				victima.mu.Lock()
				if victima.marcado.Load() { // otro Remove se adelantó
					victima.mu.Unlock()
					return false
				}
				victima.marcado.Store(true) // linealización de Remove: borrado LÓGICO
				marcada = true
			}
			masAlto, valido := -1, true
			var prev *nodo[K]
			for nivel := 0; valido && nivel <= alto; nivel++ {
				pred := preds[nivel]
				if pred != prev {
					pred.mu.Lock()
					masAlto, prev = nivel, pred
				}
				valido = !pred.marcado.Load() && pred.sig[nivel].Load() == victima
			}
			if !valido {
				soltar(&preds, masAlto)
				continue
			}
			for nivel := alto; nivel >= 0; nivel-- { // borrado FÍSICO, de arriba abajo
				preds[nivel].sig[nivel].Store(victima.sig[nivel].Load())
			}
			victima.mu.Unlock()
			soltar(&preds, masAlto)
			return true
		}
		return false
	}
}

func puedeBorrarse[K cmp.Ordered](n *nodo[K], nivelEncontrado int) bool {
	return n.enlazado.Load() && n.alto == nivelEncontrado && !n.marcado.Load()
}

// All recorre las claves en orden. Es "débilmente consistente": ve las claves presentes durante
// todo el recorrido; las que se añaden o quitan mientras tanto pueden verse o no.
func (l *Lista[K]) All() iter.Seq[K] {
	return func(yield func(K) bool) {
		for n := l.cabeza.sig[0].Load(); n.centinela == 0; n = n.sig[0].Load() {
			if n.enlazado.Load() && !n.marcado.Load() && !yield(n.clave) {
				return
			}
		}
	}
}
