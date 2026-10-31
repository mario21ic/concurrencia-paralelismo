package main

import "sync"

// RW es la interfaz común de los cerrojos lector-escritor comparados.
type RW interface {
	RLock()
	RUnlock()
	Lock()
	Unlock()
}

// ---- 1. Preferencia de lectores: el primer lector cierra la sala, el último la abre ----

// PrefLectores deja entrar a un lector mientras haya otros leyendo, aunque un escritor espere:
// con lectores continuos el escritor NO entra nunca (inanición del escritor).
type PrefLectores struct {
	mu   sync.Mutex // protege n
	n    int
	sala sync.Mutex // "la sala está ocupada por escritores o por el grupo de lectores"
}

func (l *PrefLectores) RLock() {
	l.mu.Lock()
	if l.n++; l.n == 1 {
		l.sala.Lock()
	}
	l.mu.Unlock()
}
func (l *PrefLectores) RUnlock() {
	l.mu.Lock()
	if l.n--; l.n == 0 {
		l.sala.Unlock()
	}
	l.mu.Unlock()
}
func (l *PrefLectores) Lock()   { l.sala.Lock() }
func (l *PrefLectores) Unlock() { l.sala.Unlock() }

// ---- 2. Preferencia de escritores (con Cond): si hay un escritor esperando, no entran más lectores ----

// PrefEscritores evita la inanición del escritor, pero con escritores continuos los lectores no entran.
type PrefEscritores struct {
	mu          sync.Mutex
	c           *sync.Cond
	lectores    int
	escribiendo bool
	escEsperan  int
}

func NuevoPrefEscritores() *PrefEscritores {
	l := &PrefEscritores{}
	l.c = sync.NewCond(&l.mu)
	return l
}

func (l *PrefEscritores) RLock() {
	l.mu.Lock()
	for l.escribiendo || l.escEsperan > 0 {
		l.c.Wait()
	}
	l.lectores++
	l.mu.Unlock()
}
func (l *PrefEscritores) RUnlock() {
	l.mu.Lock()
	if l.lectores--; l.lectores == 0 {
		l.c.Broadcast()
	}
	l.mu.Unlock()
}
func (l *PrefEscritores) Lock() {
	l.mu.Lock()
	l.escEsperan++
	for l.escribiendo || l.lectores > 0 {
		l.c.Wait()
	}
	l.escEsperan--
	l.escribiendo = true
	l.mu.Unlock()
}
func (l *PrefEscritores) Unlock() {
	l.mu.Lock()
	l.escribiendo = false
	l.c.Broadcast()
	l.mu.Unlock()
}

// ---- 3. Justo: un torniquete por el que pasan TODOS (lectores y escritores) ----

// Justo usa un torniquete: el escritor lo cierra al llegar, así que los lectores nuevos esperan
// detrás de él; en cuanto acaba, los lectores acumulados pasan. Ninguno de los dos grupos se inanicia.
type Justo struct {
	torno sync.Mutex
	mu    sync.Mutex
	n     int
	sala  sync.Mutex
}

func (l *Justo) RLock() {
	l.torno.Lock() // pasa el torniquete (espera si hay un escritor en cola o escribiendo)
	l.torno.Unlock()
	l.mu.Lock()
	if l.n++; l.n == 1 {
		l.sala.Lock()
	}
	l.mu.Unlock()
}
func (l *Justo) RUnlock() {
	l.mu.Lock()
	if l.n--; l.n == 0 {
		l.sala.Unlock()
	}
	l.mu.Unlock()
}
func (l *Justo) Lock() {
	l.torno.Lock() // cierra el torniquete: no entran lectores nuevos
	l.sala.Lock()  // espera a que salgan los lectores actuales
}
func (l *Justo) Unlock() {
	l.sala.Unlock()
	l.torno.Unlock()
}

// Estándar envuelve sync.RWMutex para compararlo.
type Estandar struct{ sync.RWMutex }
