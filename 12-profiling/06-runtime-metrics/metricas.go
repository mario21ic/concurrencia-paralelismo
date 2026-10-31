package main

import (
	"math"
	"runtime/metrics"
	"time"
)

// Nombres de las métricas usadas (ver metrics.All() para el catálogo completo).
const (
	mGoroutines = "/sched/goroutines:goroutines"
	mRunnable   = "/sched/goroutines/runnable:goroutines"
	mWaiting    = "/sched/goroutines/waiting:goroutines"
	mRunning    = "/sched/goroutines/running:goroutines"
	mMaxProcs   = "/sched/gomaxprocs:threads"
	mLatencias  = "/sched/latencies:seconds"
	mMutexWait  = "/sync/mutex/wait/total:seconds"
	mCPUTotal   = "/cpu/classes/total:cpu-seconds"
	mCPUIdle    = "/cpu/classes/idle:cpu-seconds"
	mCPUUser    = "/cpu/classes/user:cpu-seconds"
	mCPUGC      = "/cpu/classes/gc/total:cpu-seconds"
	mGCCiclos   = "/gc/cycles/total:gc-cycles"
)

// Foto es una lectura de las métricas relevantes.
type Foto struct {
	Goroutines, Runnable, Waiting, Running, MaxProcs uint64
	MutexWait                                        float64 // segundos acumulados esperando mutex
	CPUTotal, CPUIdle, CPUUser, CPUGC                float64
	GCCiclos                                         uint64
	Latencias                                        *metrics.Float64Histogram
	Hora                                             time.Time
}

var nombres = []string{mGoroutines, mRunnable, mWaiting, mRunning, mMaxProcs, mLatencias,
	mMutexWait, mCPUTotal, mCPUIdle, mCPUUser, mCPUGC, mGCCiclos}

// Leer toma una foto. metrics.Read es barato y seguro para usar en producción.
func Leer() Foto {
	ms := make([]metrics.Sample, len(nombres))
	for i, n := range nombres {
		ms[i].Name = n
	}
	metrics.Read(ms)
	f := Foto{Hora: time.Now()}
	for _, m := range ms {
		switch m.Name {
		case mGoroutines:
			f.Goroutines = m.Value.Uint64()
		case mRunnable:
			f.Runnable = m.Value.Uint64()
		case mWaiting:
			f.Waiting = m.Value.Uint64()
		case mRunning:
			f.Running = m.Value.Uint64()
		case mMaxProcs:
			f.MaxProcs = m.Value.Uint64()
		case mLatencias:
			f.Latencias = m.Value.Float64Histogram()
		case mMutexWait:
			f.MutexWait = m.Value.Float64()
		case mCPUTotal:
			f.CPUTotal = m.Value.Float64()
		case mCPUIdle:
			f.CPUIdle = m.Value.Float64()
		case mCPUUser:
			f.CPUUser = m.Value.Float64()
		case mCPUGC:
			f.CPUGC = m.Value.Float64()
		case mGCCiclos:
			f.GCCiclos = m.Value.Uint64()
		}
	}
	return f
}

// RestarHistograma devuelve b - a (las observaciones ocurridas entre ambas fotos).
// Los histogramas de runtime/metrics son ACUMULATIVOS desde el arranque.
func RestarHistograma(b, a *metrics.Float64Histogram) *metrics.Float64Histogram {
	d := &metrics.Float64Histogram{Buckets: b.Buckets, Counts: make([]uint64, len(b.Counts))}
	for i := range b.Counts {
		d.Counts[i] = b.Counts[i]
		if a != nil && i < len(a.Counts) {
			d.Counts[i] -= a.Counts[i]
		}
	}
	return d
}

// Cuantil devuelve el límite SUPERIOR del cubo donde cae el cuantil q (0..1);
// es una cota, no un valor exacto (el histograma agrupa observaciones).
// Devuelve 0 si no hay observaciones.
func Cuantil(h *metrics.Float64Histogram, q float64) float64 {
	var total uint64
	for _, c := range h.Counts {
		total += c
	}
	if total == 0 {
		return 0
	}
	objetivo := uint64(math.Ceil(q * float64(total)))
	var acum uint64
	for i, c := range h.Counts {
		acum += c
		if acum >= objetivo {
			sup := h.Buckets[i+1]
			if math.IsInf(sup, 1) { // último cubo abierto: usar su límite inferior
				return h.Buckets[i]
			}
			return sup
		}
	}
	return h.Buckets[len(h.Buckets)-1]
}

// Observaciones suma los recuentos.
func Observaciones(h *metrics.Float64Histogram) (n uint64) {
	for _, c := range h.Counts {
		n += c
	}
	return
}
