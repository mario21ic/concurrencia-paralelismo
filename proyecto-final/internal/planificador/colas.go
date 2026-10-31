package planificador

import (
	"container/heap"
	"time"
)

// ---- Cola FIFO por nivel de prioridad ----

type item struct {
	id    string
	gen   uint64    // generación de la entrada cuando se encoló: descarta elementos obsoletos
	desde time.Time // cuándo se encoló (para el envejecimiento)
}

// fifo es una cola de enteros sobre un slice con índice de cabeza (amortizado O(1)).
type fifo struct {
	items []item
	ini   int
}

func (f *fifo) push(it item) { f.items = append(f.items, it) }
func (f *fifo) peek() (item, bool) {
	if f.ini >= len(f.items) {
		return item{}, false
	}
	return f.items[f.ini], true
}
func (f *fifo) pop() {
	f.items[f.ini] = item{} // no retener la cadena del id
	f.ini++
	if f.ini > 64 && f.ini*2 >= len(f.items) { // compacta cuando más de la mitad está consumida
		f.items = append(f.items[:0], f.items[f.ini:]...)
		f.ini = 0
	}
}

// ---- Línea de tiempo: eventos futuros (reintentos, caducidad de arriendos, purgas) ----

type tipoEvento uint8

const (
	evReintento tipoEvento = iota // un trabajo retardado vuelve a la cola
	evArriendo                    // vence el arriendo de un trabajo en curso
	evPurga                       // se elimina un trabajo terminado
)

type evento struct {
	at   time.Time
	tipo tipoEvento
	id   string
	gen  uint64
}

// lineaTiempo es un min-heap por instante. Los eventos no se borran: se invalidan por generación
// ("lazy deletion") cuando la entrada cambia de estado, y se descartan al llegar su hora.
type lineaTiempo []evento

func (l lineaTiempo) Len() int           { return len(l) }
func (l lineaTiempo) Less(i, j int) bool { return l[i].at.Before(l[j].at) }
func (l lineaTiempo) Swap(i, j int)      { l[i], l[j] = l[j], l[i] }
func (l *lineaTiempo) Push(x any)        { *l = append(*l, x.(evento)) }
func (l *lineaTiempo) Pop() any {
	old := *l
	n := len(old)
	x := old[n-1]
	*l = old[:n-1]
	return x
}

func (l *lineaTiempo) añadir(e evento) (esLaPrimera bool) {
	heap.Push(l, e)
	return (*l)[0].id == e.id && (*l)[0].at.Equal(e.at) && (*l)[0].tipo == e.tipo
}
func (l *lineaTiempo) sacar() evento { return heap.Pop(l).(evento) }
