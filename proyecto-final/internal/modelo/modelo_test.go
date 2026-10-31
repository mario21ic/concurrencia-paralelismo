package modelo

import (
	"errors"
	"testing"
)

func TestValidarAplicaDefectosYRechazaInvalidos(t *testing.T) {
	s := Solicitud{Tipo: "x"}
	if err := s.Validar(5); err != nil || s.Tenant != "anon" || s.MaxIntentos != 5 || s.TimeoutMS != 30_000 {
		t.Fatalf("defectos: %+v err=%v", s, err)
	}
	for _, malo := range []Solicitud{{}, {Tipo: "x", Prioridad: -1}, {Tipo: "x", Prioridad: Niveles}} {
		if err := malo.Validar(3); !errors.Is(err, ErrSolicitud) {
			t.Errorf("%+v debía ser inválida, err=%v", malo, err)
		}
	}
}

func TestEstadosTerminales(t *testing.T) {
	for e, esperado := range map[Estado]bool{Pendiente: false, Retardado: false, EnCurso: false, Completado: true, Fallido: true, Cancelado: true} {
		if e.Terminal() != esperado {
			t.Errorf("%s.Terminal() = %v", e, e.Terminal())
		}
	}
}
