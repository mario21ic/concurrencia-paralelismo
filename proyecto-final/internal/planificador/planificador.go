// Package planificador es el corazón del sistema: una cola de trabajos con prioridades, envejecimiento,
// arriendos con caducidad, reintentos con backoff, límite de tasa por tenant, idempotencia y
// contrapresión. Es un componente en memoria, seguro para uso concurrente, sin goroutines propias salvo
// Run (el reloj que vence reintentos y arriendos), que gestiona quien lo use (patrón Run(ctx), módulo 15).
package planificador

import (
	"container/heap"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	mrand "math/rand/v2"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/mario21ic/golang-concurrencia-paralelismo/proyecto-final/internal/modelo"
)

// Config agrupa los parámetros del planificador. El valor cero de cada campo toma un defecto razonable.
type Config struct {
	Capacidad      int           // máximo de trabajos en espera (pendientes + retardados); defecto 10 000
	Envejecimiento time.Duration // cada esta espera, un trabajo "sube" un nivel efectivo; 0 = prioridad estricta
	ArriendoTTL    time.Duration // vigencia de un arriendo sin latidos; defecto 10 s
	BackoffBase    time.Duration // espera tras el 1.er fallo; se duplica en cada intento; defecto 100 ms
	BackoffMax     time.Duration // tope del backoff; defecto 30 s
	MaxIntentos    int           // intentos por defecto de un trabajo; defecto 3
	TasaTenant     rate.Limit    // envíos/s permitidos por tenant; 0 = sin límite
	RafagaTenant   int           // ráfaga del cubo de fichas por tenant; defecto 10
	Retencion      time.Duration // cuánto se conserva un trabajo terminado; defecto 10 min
	// Jitter transforma el backoff calculado. Defecto: "equal jitter" (mitad fija + mitad aleatoria),
	// que evita que muchos reintentos coincidan (estampida). Los tests lo sustituyen por la identidad.
	Jitter func(time.Duration) time.Duration
}

func (c *Config) porDefecto() {
	if c.Capacidad <= 0 {
		c.Capacidad = 10_000
	}
	if c.ArriendoTTL <= 0 {
		c.ArriendoTTL = 10 * time.Second
	}
	if c.BackoffBase <= 0 {
		c.BackoffBase = 100 * time.Millisecond
	}
	if c.BackoffMax <= 0 {
		c.BackoffMax = 30 * time.Second
	}
	if c.MaxIntentos <= 0 {
		c.MaxIntentos = 3
	}
	if c.RafagaTenant <= 0 {
		c.RafagaTenant = 10
	}
	if c.Retencion <= 0 {
		c.Retencion = 10 * time.Minute
	}
	if c.Jitter == nil {
		c.Jitter = func(d time.Duration) time.Duration {
			if d <= 1 {
				return d
			}
			return d/2 + time.Duration(mrand.Int64N(int64(d/2)+1))
		}
	}
}

// entrada es el estado interno de un trabajo.
type entrada struct {
	job      modelo.Job
	gen      uint64    // se incrementa en cada transición: invalida elementos y eventos antiguos
	encolado time.Time // último instante en que pasó a pendiente
	hecho    chan struct{}
}

// Planificador gestiona todos los trabajos.
type Planificador struct {
	cfg Config
	M   Contadores

	mu       sync.Mutex
	trabajos map[string]*entrada
	claves   map[string]string // clave de idempotencia → id
	colas    [modelo.Niveles]fifo
	tiempo   lineaTiempo
	cuenta   map[modelo.Estado]int
	porNivel [modelo.Niveles]int
	enEspera int // pendientes + retardados
	limites  map[string]*rate.Limiter

	esperando int           // goroutines bloqueadas en Arrendar
	cambio    chan struct{} // se cierra (y se reemplaza) al llegar trabajo: difusión compatible con ctx
	despertar chan struct{} // avisa al reloj de que hay un evento más próximo

	prefijo string
	seq     uint64
}

// Nuevo crea un planificador. Hay que ejecutar Run(ctx) en una goroutine para que venzan los
// reintentos y los arriendos.
func Nuevo(cfg Config) *Planificador {
	cfg.porDefecto()
	var b [4]byte
	rand.Read(b[:])
	return &Planificador{
		cfg:       cfg,
		trabajos:  map[string]*entrada{},
		claves:    map[string]string{},
		cuenta:    map[modelo.Estado]int{},
		limites:   map[string]*rate.Limiter{},
		cambio:    make(chan struct{}),
		despertar: make(chan struct{}, 1),
		prefijo:   hex.EncodeToString(b[:]),
	}
}

// Config devuelve la configuración efectiva.
func (p *Planificador) Config() Config { return p.cfg }

// ---- Envío ----

// Enviar admite un trabajo nuevo. Devuelve creado=false si la clave de idempotencia ya existía (y el
// trabajo existente). Errores: ErrSolicitud, ErrColaLlena (contrapresión), *ErrLimite (cuota del tenant).
func (p *Planificador) Enviar(s modelo.Solicitud) (job modelo.Job, creado bool, err error) {
	if err := s.Validar(p.cfg.MaxIntentos); err != nil {
		return modelo.Job{}, false, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()

	if s.Clave != "" { // un reenvío idempotente no gasta cuota ni capacidad
		if id, ok := p.claves[s.Clave]; ok {
			if e, ok := p.trabajos[id]; ok {
				p.M.Duplicados.Add(1)
				return e.job, false, nil
			}
		}
	}
	if p.enEspera >= p.cfg.Capacidad {
		p.M.RechazadosLlena.Add(1)
		return modelo.Job{}, false, modelo.ErrColaLlena
	}
	if lim := p.limitador(s.Tenant); lim != nil {
		r := lim.Reserve()
		if d := r.Delay(); d > 0 || !r.OK() {
			r.Cancel() // no consume la ficha
			p.M.RechazadosTasa.Add(1)
			return modelo.Job{}, false, &modelo.ErrLimite{RetryAfter: max(d, time.Millisecond)}
		}
	}

	ahora := time.Now()
	p.seq++
	e := &entrada{hecho: make(chan struct{})}
	e.job = modelo.Job{
		ID: fmt.Sprintf("%s-%d", p.prefijo, p.seq), Tipo: s.Tipo, Payload: s.Payload, Prioridad: s.Prioridad,
		Tenant: s.Tenant, Clave: s.Clave, MaxIntentos: s.MaxIntentos, TimeoutMS: s.TimeoutMS, Creado: ahora,
	}
	p.trabajos[e.job.ID] = e
	if s.Clave != "" {
		p.claves[s.Clave] = e.job.ID
	}
	p.M.Enviados.Add(1)
	p.encolar(e, ahora)
	return e.job, true, nil
}

func (p *Planificador) limitador(tenant string) *rate.Limiter {
	if p.cfg.TasaTenant <= 0 {
		return nil
	}
	l, ok := p.limites[tenant]
	if !ok {
		l = rate.NewLimiter(p.cfg.TasaTenant, p.cfg.RafagaTenant)
		p.limites[tenant] = l
	}
	return l
}

// ---- Transiciones (todas con p.mu tomado) ----

func (p *Planificador) mover(e *entrada, nuevo modelo.Estado) {
	viejo := e.job.Estado
	p.cuenta[viejo]--
	p.cuenta[nuevo]++
	if viejo == modelo.Pendiente {
		p.porNivel[e.job.Prioridad]--
	}
	if nuevo == modelo.Pendiente {
		p.porNivel[e.job.Prioridad]++
	}
	if viejo == modelo.Pendiente || viejo == modelo.Retardado {
		p.enEspera--
	}
	if nuevo == modelo.Pendiente || nuevo == modelo.Retardado {
		p.enEspera++
	}
	e.job.Estado = nuevo
	e.gen++
}

func (p *Planificador) encolar(e *entrada, ahora time.Time) {
	p.mover(e, modelo.Pendiente)
	e.encolado = ahora
	p.colas[e.job.Prioridad].push(item{id: e.job.ID, gen: e.gen, desde: ahora})
	if p.esperando > 0 { // despierta a TODOS los que esperan trabajo
		close(p.cambio)
		p.cambio = make(chan struct{})
	}
}

func (p *Planificador) programar(ev evento) {
	if p.tiempo.añadir(ev) {
		select {
		case p.despertar <- struct{}{}:
		default:
		}
	}
}

func (p *Planificador) terminar(e *entrada, estado modelo.Estado, ahora time.Time) {
	p.mover(e, estado)
	e.job.Fin = ahora
	e.job.Worker = ""
	close(e.hecho)
	p.programar(evento{at: ahora.Add(p.cfg.Retencion), tipo: evPurga, id: e.job.ID})
	p.M.Total.Observar(ahora.Sub(e.job.Creado))
}

// fallar decide entre reintentar (con backoff) o enviar el trabajo a la cola de muertos.
func (p *Planificador) fallar(e *entrada, motivo string, reintentable bool, ahora time.Time) {
	e.job.Error = motivo
	e.job.Worker = ""
	if reintentable && e.job.Intento < e.job.MaxIntentos {
		d := p.backoff(e.job.Intento)
		p.mover(e, modelo.Retardado)
		p.M.Reintentos.Add(1)
		p.programar(evento{at: ahora.Add(d), tipo: evReintento, id: e.job.ID, gen: e.gen})
		return
	}
	p.M.Fallidos.Add(1)
	p.terminar(e, modelo.Fallido, ahora)
}

// backoff exponencial con tope y jitter: base·2^(intento-1).
func (p *Planificador) backoff(intento int) time.Duration {
	d := p.cfg.BackoffBase
	for i := 1; i < intento && d < p.cfg.BackoffMax; i++ {
		d *= 2
	}
	return p.cfg.Jitter(min(d, p.cfg.BackoffMax))
}

// ---- Arriendo ----

// Arrendar entrega hasta max trabajos a un worker, eligiendo siempre el más urgente (ver siguiente).
// Si no hay ninguno espera hasta `espera` (long polling) o hasta que ctx se cancele; en ambos casos
// devuelve una lista vacía. Cada trabajo arrendado debe terminar con Ack/Nack antes de que venza el
// arriendo (renovable con Latido); si no, vuelve a la cola.
func (p *Planificador) Arrendar(ctx context.Context, worker string, max int, espera time.Duration) ([]modelo.Job, error) {
	if max <= 0 {
		max = 1
	}
	var temporizador *time.Timer
	var tc <-chan time.Time
	for {
		p.mu.Lock()
		jobs := p.tomar(worker, max, time.Now())
		if len(jobs) > 0 || espera <= 0 {
			p.mu.Unlock()
			return jobs, nil
		}
		p.esperando++
		ch := p.cambio
		p.mu.Unlock()

		if temporizador == nil {
			temporizador = time.NewTimer(espera)
			defer temporizador.Stop()
			tc = temporizador.C
		}
		var salida error
		select {
		case <-ch:
		case <-tc:
			salida = errSinTrabajo
		case <-ctx.Done():
			salida = ctx.Err()
		}
		p.mu.Lock()
		p.esperando--
		p.mu.Unlock()
		switch salida {
		case nil:
		case errSinTrabajo:
			return nil, nil
		default:
			return nil, salida
		}
	}
}

var errSinTrabajo = fmt.Errorf("sin trabajo")

func (p *Planificador) tomar(worker string, max int, ahora time.Time) []modelo.Job {
	var out []modelo.Job
	for len(out) < max {
		e := p.siguiente(ahora)
		if e == nil {
			break
		}
		p.M.Espera.Observar(ahora.Sub(e.encolado))
		p.mover(e, modelo.EnCurso)
		e.job.Intento++
		e.job.Worker = worker
		e.job.Inicio = ahora
		p.programar(evento{at: ahora.Add(p.cfg.ArriendoTTL), tipo: evArriendo, id: e.job.ID, gen: e.gen})
		p.M.Arrendados.Add(1)
		j := e.job
		j.ArriendoMS = int(p.cfg.ArriendoTTL / time.Millisecond)
		out = append(out, j)
	}
	return out
}

// siguiente elige entre las cabezas de las colas por nivel la de mayor prioridad EFECTIVA:
// nivel + espera/Envejecimiento. Dentro de un nivel, FIFO. Sin envejecimiento la prioridad es estricta
// y un flujo continuo de trabajos urgentes puede dejar sin servicio a los de baja prioridad (inanición,
// módulo 15); con envejecimiento, la espera acaba compensando la diferencia de nivel.
func (p *Planificador) siguiente(ahora time.Time) *entrada {
	mejor := -1
	var mejorEf float64
	var mejorE *entrada
	for nivel := modelo.Niveles - 1; nivel >= 0; nivel-- {
		q := &p.colas[nivel]
		var e *entrada
		for { // descarta elementos obsoletos (cancelados, reencolados…)
			it, ok := q.peek()
			if !ok {
				break
			}
			if x := p.trabajos[it.id]; x != nil && x.job.Estado == modelo.Pendiente && x.gen == it.gen {
				e = x
				break
			}
			q.pop()
		}
		if e == nil {
			continue
		}
		ef := float64(nivel)
		if p.cfg.Envejecimiento > 0 {
			ef += float64(ahora.Sub(e.encolado)) / float64(p.cfg.Envejecimiento)
		}
		if mejor == -1 || ef > mejorEf {
			mejor, mejorEf, mejorE = nivel, ef, e
		}
	}
	if mejorE != nil {
		p.colas[mejor].pop()
	}
	return mejorE
}

// arrendado verifica que `worker` sigue siendo el arrendatario vigente de `id`.
func (p *Planificador) arrendado(id, worker string) (*entrada, error) {
	e, ok := p.trabajos[id]
	if !ok {
		return nil, modelo.ErrNoEncontrado
	}
	if e.job.Estado != modelo.EnCurso || e.job.Worker != worker {
		return nil, modelo.ErrArriendoPerdido
	}
	return e, nil
}

// Ack marca el trabajo como completado.
func (p *Planificador) Ack(id, worker string, resultado []byte) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	e, err := p.arrendado(id, worker)
	if err != nil {
		return err
	}
	ahora := time.Now()
	e.job.Resultado = resultado
	e.job.Error = ""
	p.M.Ejecucion.Observar(ahora.Sub(e.job.Inicio))
	p.M.Completados.Add(1)
	p.terminar(e, modelo.Completado, ahora)
	return nil
}

// Nack comunica un fallo. Si es reintentable y quedan intentos, el trabajo se retarda con backoff.
func (p *Planificador) Nack(id, worker, motivo string, reintentable bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	e, err := p.arrendado(id, worker)
	if err != nil {
		return err
	}
	ahora := time.Now()
	p.M.Ejecucion.Observar(ahora.Sub(e.job.Inicio))
	p.fallar(e, motivo, reintentable, ahora)
	return nil
}

// Latido renueva el arriendo (un trabajo largo debe latir más a menudo que ArriendoTTL).
func (p *Planificador) Latido(id, worker string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	e, err := p.arrendado(id, worker)
	if err != nil {
		return err
	}
	e.gen++ // invalida el evento de caducidad anterior
	p.programar(evento{at: time.Now().Add(p.cfg.ArriendoTTL), tipo: evArriendo, id: id, gen: e.gen})
	return nil
}

// Cancelar anula un trabajo que aún no ha empezado (pendiente o retardado).
func (p *Planificador) Cancelar(id string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	e, ok := p.trabajos[id]
	if !ok {
		return modelo.ErrNoEncontrado
	}
	if e.job.Estado != modelo.Pendiente && e.job.Estado != modelo.Retardado {
		return fmt.Errorf("%w: está %s", modelo.ErrEstadoInvalido, e.job.Estado)
	}
	p.M.Cancelados.Add(1)
	p.terminar(e, modelo.Cancelado, time.Now())
	return nil
}

// ---- Consultas ----

func (p *Planificador) Obtener(id string) (modelo.Job, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	e, ok := p.trabajos[id]
	if !ok {
		return modelo.Job{}, false
	}
	return e.job, true
}

// Esperar bloquea hasta que el trabajo termine, venza `max` o se cancele ctx; devuelve su estado actual.
func (p *Planificador) Esperar(ctx context.Context, id string, max time.Duration) (modelo.Job, error) {
	p.mu.Lock()
	e, ok := p.trabajos[id]
	p.mu.Unlock()
	if !ok {
		return modelo.Job{}, modelo.ErrNoEncontrado
	}
	t := time.NewTimer(max)
	defer t.Stop()
	select {
	case <-e.hecho:
	case <-t.C:
	case <-ctx.Done():
		return modelo.Job{}, ctx.Err()
	}
	j, _ := p.Obtener(id)
	return j, nil
}

// Estadisticas es una instantánea del estado interno.
type Estadisticas struct {
	Estados    map[modelo.Estado]int         `json:"estados"`
	PorNivel   [modelo.Niveles]int           `json:"pendientes_por_prioridad"`
	EnEspera   int                           `json:"en_espera"`
	Esperando  int                           `json:"workers_esperando"`
	Totales    map[string]int64              `json:"totales"`
	LatenciaMS map[string]map[string]float64 `json:"latencia_ms"`
}

func (p *Planificador) Estadisticas() Estadisticas {
	p.mu.Lock()
	st := Estadisticas{Estados: map[modelo.Estado]int{}, PorNivel: p.porNivel, EnEspera: p.enEspera, Esperando: p.esperando}
	for k, v := range p.cuenta {
		if k != "" && v != 0 {
			st.Estados[k] = v
		}
	}
	p.mu.Unlock()
	m := &p.M
	st.Totales = map[string]int64{
		"enviados": m.Enviados.Load(), "duplicados": m.Duplicados.Load(),
		"rechazados_cola_llena": m.RechazadosLlena.Load(), "rechazados_tasa": m.RechazadosTasa.Load(),
		"arrendados": m.Arrendados.Load(), "completados": m.Completados.Load(), "reintentos": m.Reintentos.Load(),
		"fallidos": m.Fallidos.Load(), "arriendos_caducados": m.Caducados.Load(), "cancelados": m.Cancelados.Load(),
	}
	ms := func(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }
	st.LatenciaMS = map[string]map[string]float64{}
	for nombre, h := range map[string]*Histograma{"espera": &m.Espera, "ejecucion": &m.Ejecucion, "total": &m.Total} {
		st.LatenciaMS[nombre] = map[string]float64{
			"media": ms(h.Media()), "p50": ms(h.Percentil(.5)), "p95": ms(h.Percentil(.95)), "p99": ms(h.Percentil(.99)),
		}
	}
	return st
}

// ---- Reloj ----

// Run hace vencer los eventos programados: reintentos (retardado → pendiente), arriendos caducados
// (el worker desapareció) y purga de trabajos terminados. Bloquea hasta que ctx se cancela.
func (p *Planificador) Run(ctx context.Context) error {
	t := time.NewTimer(time.Hour)
	defer t.Stop()
	for {
		p.mu.Lock()
		ahora := time.Now()
		for len(p.tiempo) > 0 && !p.tiempo[0].at.After(ahora) {
			p.procesar(p.tiempo.sacar(), ahora)
		}
		espera := time.Hour
		if len(p.tiempo) > 0 {
			espera = p.tiempo[0].at.Sub(ahora)
		}
		p.mu.Unlock()
		t.Reset(espera)
		select {
		case <-t.C:
		case <-p.despertar:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (p *Planificador) procesar(ev evento, ahora time.Time) {
	e, ok := p.trabajos[ev.id]
	if !ok {
		return
	}
	switch ev.tipo {
	case evReintento:
		if e.job.Estado == modelo.Retardado && e.gen == ev.gen {
			p.encolar(e, ahora)
		}
	case evArriendo:
		if e.job.Estado == modelo.EnCurso && e.gen == ev.gen {
			p.M.Caducados.Add(1)
			p.M.Ejecucion.Observar(ahora.Sub(e.job.Inicio))
			p.fallar(e, "arriendo caducado: el worker no respondió", true, ahora)
		}
	case evPurga:
		if e.job.Estado.Terminal() {
			delete(p.trabajos, ev.id)
			if e.job.Clave != "" && p.claves[e.job.Clave] == ev.id {
				delete(p.claves, e.job.Clave)
			}
			p.cuenta[e.job.Estado]--
		}
	}
}

var _ heap.Interface = (*lineaTiempo)(nil)
