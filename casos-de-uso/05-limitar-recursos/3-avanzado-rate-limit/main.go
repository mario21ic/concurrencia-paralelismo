// Caso 5.3 (avanzado): limitar la TASA de peticiones por cliente (token bucket) y decir cuándo reintentar.
//
// Problema: una API pública debe aceptar como mucho 2 peticiones/s por cliente, con ráfagas de hasta 5.
// Un cliente abusivo no debe afectar a los demás, y la respuesta de rechazo debe indicar CUÁNDO
// volver a intentarlo (429 + Retry-After) para que los clientes bien escritos esperen en vez de insistir.
//
// Diferencia con el semáforo (5.1): el semáforo limita la CONCURRENCIA (cuántas a la vez); el rate
// limiter limita la TASA (cuántas por segundo), aunque cada una dure microsegundos.
//
// Herramientas: golang.org/x/time/rate (token bucket), map protegido por Mutex, poda de clientes ociosos.
//
//	go run .
package main

import (
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type cliente struct {
	lim   *rate.Limiter
	visto time.Time
}

type Limitador struct {
	tasa   rate.Limit
	rafaga int
	mu     sync.Mutex
	por    map[string]*cliente
}

func NuevoLimitador(tasa rate.Limit, rafaga int) *Limitador {
	return &Limitador{tasa: tasa, rafaga: rafaga, por: map[string]*cliente{}}
}

// Permitir consume una ficha del cliente. Si no hay, NO la consume y dice cuánto esperar.
func (l *Limitador) Permitir(id string) (ok bool, esperar time.Duration) {
	l.mu.Lock()
	c, existe := l.por[id]
	if !existe {
		c = &cliente{lim: rate.NewLimiter(l.tasa, l.rafaga)}
		l.por[id] = c
	}
	c.visto = time.Now()
	l.mu.Unlock()

	r := c.lim.Reserve() // el limiter es seguro para uso concurrente; no hace falta el cerrojo del mapa
	if d := r.Delay(); d > 0 {
		r.Cancel() // devolver la reserva: un rechazo no debe gastar ficha
		return false, d
	}
	return true, 0
}

// Podar elimina los clientes que no han aparecido en `ocioso` (evita que el mapa crezca sin límite
// con clientes de un solo uso). Se llama periódicamente.
func (l *Limitador) Podar(ocioso time.Duration) (eliminados int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for id, c := range l.por {
		if time.Since(c.visto) > ocioso {
			delete(l.por, id)
			eliminados++
		}
	}
	return
}

func (l *Limitador) Clientes() int { l.mu.Lock(); defer l.mu.Unlock(); return len(l.por) }

// Middleware aplica el límite por la cabecera X-Cliente (en producción: IP o clave de API).
func (l *Limitador) Middleware(sig http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ok, espera := l.Permitir(r.Header.Get("X-Cliente")); !ok {
			w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(espera.Seconds()))))
			http.Error(w, "demasiadas peticiones", http.StatusTooManyRequests)
			return
		}
		sig.ServeHTTP(w, r)
	})
}

func main() {
	l := NuevoLimitador(2, 5) // 2 por segundo, ráfaga de 5
	h := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) }))
	pedir := func(cli string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Cliente", cli)
		h.ServeHTTP(rec, req)
		return rec
	}
	fmt.Println("cliente «abusivo» lanza 8 peticiones seguidas (ráfaga 5, 2/s):")
	for i := 1; i <= 8; i++ {
		r := pedir("abusivo")
		fmt.Printf("  #%d → %d  Retry-After=%q\n", i, r.Code, r.Header().Get("Retry-After"))
	}
	fmt.Printf("cliente «tranquilo» a la vez → %d (su cuota es independiente)\n", pedir("tranquilo").Code)
	time.Sleep(600 * time.Millisecond) // 2/s × 0,6 s ≈ 1 ficha
	fmt.Printf("«abusivo» tras esperar 0,6 s → %d\n", pedir("abusivo").Code)
}
