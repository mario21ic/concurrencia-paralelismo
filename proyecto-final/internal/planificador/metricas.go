package planificador

import (
	"sync/atomic"
	"time"
)

// limitesMS son las cotas superiores (en ms) de los cubos del histograma.
var limitesMS = [...]float64{1, 2, 5, 10, 20, 50, 100, 200, 500, 1000, 2000, 5000, 10000, 30000}

// Histograma acumula duraciones sin cerrojos (un contador atómico por cubo): se puede observar desde
// cualquier goroutine sin contención apreciable y leer en cualquier momento.
type Histograma struct {
	cubos [len(limitesMS) + 1]atomic.Int64 // el último es +Inf
	n     atomic.Int64
	sumaN atomic.Int64 // suma en nanosegundos
}

func (h *Histograma) Observar(d time.Duration) {
	ms := float64(d) / float64(time.Millisecond)
	i := 0
	for i < len(limitesMS) && ms > limitesMS[i] {
		i++
	}
	h.cubos[i].Add(1)
	h.n.Add(1)
	h.sumaN.Add(int64(d))
}

// Cuenta devuelve cuántas observaciones hay.
func (h *Histograma) Cuenta() int64 { return h.n.Load() }

// Media de las observaciones.
func (h *Histograma) Media() time.Duration {
	if n := h.n.Load(); n > 0 {
		return time.Duration(h.sumaN.Load() / n)
	}
	return 0
}

// Percentil devuelve la cota superior del cubo donde cae el percentil q (0..1): una aproximación
// por exceso, suficiente para vigilar latencias (la resolución es la de los cubos).
func (h *Histograma) Percentil(q float64) time.Duration {
	total := h.n.Load()
	if total == 0 {
		return 0
	}
	objetivo := int64(float64(total)*q + 0.999999)
	var acum int64
	for i := range h.cubos {
		acum += h.cubos[i].Load()
		if acum >= objetivo {
			if i < len(limitesMS) {
				return time.Duration(limitesMS[i] * float64(time.Millisecond))
			}
			return time.Duration(limitesMS[len(limitesMS)-1]*float64(time.Millisecond)) * 2
		}
	}
	return 0
}

// Cubos expone (cota en ms, acumulado) para exportar en formato Prometheus.
func (h *Histograma) Cubos() (cotas []float64, acumulado []int64) {
	var acum int64
	for i := range limitesMS {
		acum += h.cubos[i].Load()
		cotas = append(cotas, limitesMS[i])
		acumulado = append(acumulado, acum)
	}
	return
}

// Contadores del planificador (todos atómicos: se leen sin tomar el cerrojo principal).
type Contadores struct {
	Enviados, Duplicados, RechazadosLlena, RechazadosTasa atomic.Int64
	Arrendados, Completados, Reintentos, Fallidos         atomic.Int64
	Caducados, Cancelados                                 atomic.Int64

	Espera    Histograma // desde que se encola hasta que un worker lo arrienda
	Ejecucion Histograma // desde el arriendo hasta el fin
	Total     Histograma // desde la creación hasta el fin
}
