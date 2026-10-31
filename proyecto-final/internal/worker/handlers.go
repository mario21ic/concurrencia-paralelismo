package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"time"
)

// Predeterminados devuelve los handlers de demostración:
//
//	eco     devuelve el payload (sirve para medir la sobrecarga del sistema)
//	dormir  {"ms": N}                               trabajo de E/S simulado
//	hash    {"rondas": N, "semilla": "..."}         trabajo de CPU: N rondas de SHA-256 encadenadas
//	fallo   {"prob": 0.3, "ms": 10, "permanente": b} falla con una probabilidad (para probar reintentos)
//	panico  provoca un pánico (para probar la recuperación)
func Predeterminados() map[string]Handler {
	return map[string]Handler{
		"eco":    func(_ context.Context, p json.RawMessage) (json.RawMessage, error) { return p, nil },
		"dormir": dormir,
		"hash":   hash,
		"fallo":  fallo,
		"panico": func(context.Context, json.RawMessage) (json.RawMessage, error) { panic("¡boom!") },
	}
}

func decodificar(p json.RawMessage, dst any) error {
	if len(p) == 0 {
		return nil
	}
	if err := json.Unmarshal(p, dst); err != nil {
		return fmt.Errorf("%w: payload: %v", ErrPermanente, err)
	}
	return nil
}

func dormir(ctx context.Context, p json.RawMessage) (json.RawMessage, error) {
	var a struct{ MS int }
	if err := decodificar(p, &a); err != nil {
		return nil, err
	}
	t := time.NewTimer(time.Duration(a.MS) * time.Millisecond)
	defer t.Stop()
	select {
	case <-t.C:
		return json.RawMessage(`"ok"`), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// hash es CPU puro: no hay E/S ni esperas, así que el único modo de acelerarlo es repartirlo entre
// más núcleos (más ranuras) o más máquinas (más contenedores worker).
func hash(ctx context.Context, p json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Rondas  int
		Semilla string
	}
	if err := decodificar(p, &a); err != nil {
		return nil, err
	}
	h := sha256.Sum256([]byte(a.Semilla))
	for i := range a.Rondas {
		if i&0xFFF == 0 && ctx.Err() != nil { // comprueba la cancelación de vez en cuando, no en cada vuelta
			return nil, ctx.Err()
		}
		h = sha256.Sum256(h[:])
	}
	return json.Marshal(hex.EncodeToString(h[:8]))
}

func fallo(ctx context.Context, p json.RawMessage) (json.RawMessage, error) {
	var a struct {
		Prob       float64
		MS         int
		Permanente bool
	}
	if err := decodificar(p, &a); err != nil {
		return nil, err
	}
	if a.MS > 0 {
		select {
		case <-time.After(time.Duration(a.MS) * time.Millisecond):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if rand.Float64() < a.Prob {
		if a.Permanente {
			return nil, fmt.Errorf("%w: fallo simulado", ErrPermanente)
		}
		return nil, fmt.Errorf("fallo simulado")
	}
	return json.RawMessage(`"ok"`), nil
}
