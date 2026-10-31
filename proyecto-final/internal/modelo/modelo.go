// Package modelo define los tipos y errores compartidos por el coordinador, el cliente y los workers.
package modelo

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// Estado es el ciclo de vida de un trabajo:
//
//	pendiente ──arrendar──► en_curso ──ack──► completado
//	    ▲                      │
//	    │                      ├─nack/expira (quedan intentos)──► retardado ──(backoff)──┐
//	    └──────────────────────┘                                                         │
//	                           └─nack/expira (sin intentos)────► fallido (cola de muertos)
//	pendiente/retardado ──cancelar──► cancelado
type Estado string

const (
	Pendiente  Estado = "pendiente"
	Retardado  Estado = "retardado"
	EnCurso    Estado = "en_curso"
	Completado Estado = "completado"
	Fallido    Estado = "fallido"
	Cancelado  Estado = "cancelado"
)

// Terminal dice si el trabajo ya no cambiará de estado.
func (e Estado) Terminal() bool { return e == Completado || e == Fallido || e == Cancelado }

// Niveles de prioridad: 0 (baja) … Niveles-1 (crítica).
const Niveles = 4

// Job es un trabajo tal como lo ve la API.
type Job struct {
	ID          string          `json:"id"`
	Tipo        string          `json:"tipo"`
	Payload     json.RawMessage `json:"payload,omitempty"`
	Prioridad   int             `json:"prioridad"`
	Tenant      string          `json:"tenant"`
	Clave       string          `json:"clave,omitempty"` // clave de idempotencia
	MaxIntentos int             `json:"max_intentos"`
	Intento     int             `json:"intento"` // veces que se ha arrendado
	TimeoutMS   int             `json:"timeout_ms"`
	Estado      Estado          `json:"estado"`
	Worker      string          `json:"worker,omitempty"`
	Resultado   json.RawMessage `json:"resultado,omitempty"`
	Error       string          `json:"error,omitempty"`
	Creado      time.Time       `json:"creado"`
	Inicio      time.Time       `json:"inicio,omitempty"` // último arriendo
	Fin         time.Time       `json:"fin,omitempty"`
	ArriendoMS  int             `json:"arriendo_ms,omitempty"` // TTL del arriendo vigente (solo en /lease)
}

// Solicitud es el cuerpo de POST /jobs.
type Solicitud struct {
	Tipo        string          `json:"tipo"`
	Payload     json.RawMessage `json:"payload,omitempty"`
	Prioridad   int             `json:"prioridad"`
	Tenant      string          `json:"tenant,omitempty"`
	Clave       string          `json:"clave,omitempty"`
	MaxIntentos int             `json:"max_intentos,omitempty"`
	TimeoutMS   int             `json:"timeout_ms,omitempty"`
}

// Validar normaliza valores por defecto y rechaza solicitudes inválidas.
func (s *Solicitud) Validar(maxIntentosPorDefecto int) error {
	if s.Tipo == "" {
		return fmt.Errorf("%w: falta el tipo", ErrSolicitud)
	}
	if s.Prioridad < 0 || s.Prioridad >= Niveles {
		return fmt.Errorf("%w: prioridad %d fuera de [0,%d]", ErrSolicitud, s.Prioridad, Niveles-1)
	}
	if s.Tenant == "" {
		s.Tenant = "anon"
	}
	if s.MaxIntentos <= 0 {
		s.MaxIntentos = maxIntentosPorDefecto
	}
	if s.TimeoutMS <= 0 {
		s.TimeoutMS = 30_000
	}
	return nil
}

// Errores del dominio. La API los traduce a códigos HTTP y el cliente los reconstruye.
var (
	ErrSolicitud       = errors.New("solicitud inválida")
	ErrColaLlena       = errors.New("cola llena")
	ErrNoEncontrado    = errors.New("trabajo no encontrado")
	ErrArriendoPerdido = errors.New("el arriendo ya no es válido")
	ErrEstadoInvalido  = errors.New("estado inválido para esta operación")
)

// ErrLimite indica que un tenant superó su cuota; RetryAfter dice cuándo reintentar.
type ErrLimite struct{ RetryAfter time.Duration }

func (e *ErrLimite) Error() string {
	return fmt.Sprintf("límite de tasa del tenant superado (reintentar en %v)", e.RetryAfter.Round(time.Millisecond))
}
