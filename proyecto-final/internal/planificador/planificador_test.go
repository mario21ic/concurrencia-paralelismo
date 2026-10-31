package planificador

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"go.uber.org/goleak"

	"github.com/mario21ic/golang-concurrencia-paralelismo/proyecto-final/internal/modelo"
)

func TestMain(m *testing.M) { goleak.VerifyTestMain(m) }

// nuevo crea un planificador con jitter nulo (backoff exacto) y lanza su reloj atado al test.
func nuevo(t *testing.T, cfg Config) *Planificador {
	t.Helper()
	cfg.Jitter = func(d time.Duration) time.Duration { return d }
	p := Nuevo(cfg)
	go p.Run(t.Context())
	return p
}

func enviar(t *testing.T, p *Planificador, tipo string, prio int) modelo.Job {
	t.Helper()
	j, _, err := p.Enviar(modelo.Solicitud{Tipo: tipo, Prioridad: prio})
	if err != nil {
		t.Fatalf("Enviar(%s): %v", tipo, err)
	}
	return j
}

func arrendar(t *testing.T, p *Planificador, worker string, n int) []modelo.Job {
	t.Helper()
	js, err := p.Arrendar(t.Context(), worker, n, 0)
	if err != nil {
		t.Fatal(err)
	}
	return js
}

func estado(t *testing.T, p *Planificador, id string) modelo.Estado {
	t.Helper()
	j, ok := p.Obtener(id)
	if !ok {
		t.Fatalf("trabajo %s no existe", id)
	}
	return j.Estado
}

func TestPrioridadYFIFO(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := nuevo(t, Config{})
		enviar(t, p, "baja1", 0)
		enviar(t, p, "media", 2)
		enviar(t, p, "baja2", 0)
		enviar(t, p, "critica", 3)
		enviar(t, p, "media2", 2)
		var orden []string
		for _, j := range arrendar(t, p, "w", 10) {
			orden = append(orden, j.Tipo)
		}
		esperado := []string{"critica", "media", "media2", "baja1", "baja2"}
		if len(orden) != len(esperado) {
			t.Fatalf("orden = %v", orden)
		}
		for i := range esperado {
			if orden[i] != esperado[i] {
				t.Fatalf("orden = %v, esperado %v", orden, esperado)
			}
		}
	})
}

func TestEnvejecimientoEvitaInanicion(t *testing.T) {
	probar := func(env time.Duration) string {
		var primero string
		synctest.Test(t, func(t *testing.T) {
			p := nuevo(t, Config{Envejecimiento: 10 * time.Second})
			p.cfg.Envejecimiento = env
			enviar(t, p, "vieja-baja", 0)
			time.Sleep(35 * time.Second) // 3.5 niveles de envejecimiento
			enviar(t, p, "nueva-alta", 3)
			primero = arrendar(t, p, "w", 1)[0].Tipo
		})
		return primero
	}
	if got := probar(0); got != "nueva-alta" {
		t.Fatalf("sin envejecimiento: primero %q, esperado nueva-alta", got)
	}
	if got := probar(10 * time.Second); got != "vieja-baja" {
		t.Fatalf("con envejecimiento: primero %q, esperado vieja-baja", got)
	}
}

func TestReintentosConBackoffExponencial(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := nuevo(t, Config{BackoffBase: time.Second, MaxIntentos: 3})
		j := enviar(t, p, "x", 1)

		arrendar(t, p, "w", 1)
		p.Nack(j.ID, "w", "boom", true) // 1.er fallo: espera 1 s
		time.Sleep(999 * time.Millisecond)
		synctest.Wait()
		if got := estado(t, p, j.ID); got != modelo.Retardado {
			t.Fatalf("a los 999 ms: %s, esperado retardado", got)
		}
		time.Sleep(time.Millisecond)
		synctest.Wait()
		if got := estado(t, p, j.ID); got != modelo.Pendiente {
			t.Fatalf("a 1 s: %s, esperado pendiente", got)
		}

		arrendar(t, p, "w", 1)
		p.Nack(j.ID, "w", "boom", true) // 2.º fallo: espera 2 s
		time.Sleep(1999 * time.Millisecond)
		synctest.Wait()
		if got := estado(t, p, j.ID); got != modelo.Retardado {
			t.Fatalf("a 1999 ms del 2.º fallo: %s", got)
		}
		time.Sleep(time.Millisecond)
		synctest.Wait()

		arrendar(t, p, "w", 1)
		p.Nack(j.ID, "w", "boom", true) // 3.er fallo: sin más intentos
		if got := estado(t, p, j.ID); got != modelo.Fallido {
			t.Fatalf("tras agotar intentos: %s, esperado fallido", got)
		}
		fin, _ := p.Obtener(j.ID)
		if fin.Intento != 3 || fin.Error != "boom" {
			t.Fatalf("intento=%d error=%q", fin.Intento, fin.Error)
		}
		if p.M.Reintentos.Load() != 2 || p.M.Fallidos.Load() != 1 {
			t.Fatalf("reintentos=%d fallidos=%d", p.M.Reintentos.Load(), p.M.Fallidos.Load())
		}
	})
}

func TestErrorPermanenteNoSeReintenta(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := nuevo(t, Config{})
		j := enviar(t, p, "x", 0)
		arrendar(t, p, "w", 1)
		p.Nack(j.ID, "w", "datos corruptos", false)
		if estado(t, p, j.ID) != modelo.Fallido {
			t.Fatal("un error permanente debe ir directo a fallido")
		}
	})
}

func TestBackoffTieneTopeYJitter(t *testing.T) {
	p := Nuevo(Config{BackoffBase: time.Second, BackoffMax: 5 * time.Second, Jitter: func(d time.Duration) time.Duration { return d }})
	for intento, esperado := range map[int]time.Duration{1: time.Second, 2: 2 * time.Second, 3: 4 * time.Second, 4: 5 * time.Second, 30: 5 * time.Second} {
		if got := p.backoff(intento); got != esperado {
			t.Errorf("backoff(%d) = %v, esperado %v", intento, got, esperado)
		}
	}
	q := Nuevo(Config{BackoffBase: time.Second}) // jitter por defecto: entre d/2 y d
	for range 1000 {
		if d := q.backoff(3); d < 2*time.Second || d > 4*time.Second {
			t.Fatalf("jitter fuera de [2s,4s]: %v", d)
		}
	}
}

func TestArriendoCaducaYVuelveALaCola(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := nuevo(t, Config{ArriendoTTL: 5 * time.Second, BackoffBase: time.Second})
		j := enviar(t, p, "x", 0)
		arrendar(t, p, "w1", 1)
		time.Sleep(5*time.Second + time.Millisecond) // el worker "murió": no hay latidos
		synctest.Wait()
		if got := estado(t, p, j.ID); got != modelo.Retardado {
			t.Fatalf("tras caducar: %s, esperado retardado", got)
		}
		if err := p.Ack(j.ID, "w1", nil); !errors.Is(err, modelo.ErrArriendoPerdido) {
			t.Fatalf("Ack tardío = %v, esperado ErrArriendoPerdido", err)
		}
		time.Sleep(time.Second)
		synctest.Wait()
		js := arrendar(t, p, "w2", 1)
		if len(js) != 1 || js[0].Intento != 2 {
			t.Fatalf("segundo arriendo: %+v", js)
		}
		if err := p.Ack(js[0].ID, "w2", []byte(`"ok"`)); err != nil {
			t.Fatal(err)
		}
		if p.M.Caducados.Load() != 1 {
			t.Fatalf("caducados = %d", p.M.Caducados.Load())
		}
	})
}

func TestLatidoRenuevaElArriendo(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := nuevo(t, Config{ArriendoTTL: 5 * time.Second})
		j := enviar(t, p, "largo", 0)
		arrendar(t, p, "w", 1)
		for range 4 { // 12 s en total, con TTL de 5 s
			time.Sleep(3 * time.Second)
			if err := p.Latido(j.ID, "w"); err != nil {
				t.Fatalf("Latido: %v", err)
			}
		}
		if got := estado(t, p, j.ID); got != modelo.EnCurso {
			t.Fatalf("con latidos debía seguir en curso, está %s", got)
		}
		if err := p.Ack(j.ID, "w", nil); err != nil {
			t.Fatal(err)
		}
		if p.M.Caducados.Load() != 0 {
			t.Fatal("no debía caducar ningún arriendo")
		}
		if err := p.Latido(j.ID, "w"); !errors.Is(err, modelo.ErrArriendoPerdido) {
			t.Fatalf("Latido tras Ack = %v", err)
		}
	})
}

func TestSoloElArrendatarioPuedeTerminar(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := nuevo(t, Config{})
		j := enviar(t, p, "x", 0)
		arrendar(t, p, "w1", 1)
		if err := p.Ack(j.ID, "w2", nil); !errors.Is(err, modelo.ErrArriendoPerdido) {
			t.Fatalf("Ack de otro worker = %v", err)
		}
		if err := p.Ack("no-existe", "w1", nil); !errors.Is(err, modelo.ErrNoEncontrado) {
			t.Fatalf("Ack de id inexistente = %v", err)
		}
	})
}

func TestIdempotencia(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := nuevo(t, Config{})
		s := modelo.Solicitud{Tipo: "x", Clave: "pedido-42"}
		a, creadoA, _ := p.Enviar(s)
		b, creadoB, _ := p.Enviar(s)
		if !creadoA || creadoB || a.ID != b.ID {
			t.Fatalf("creados=%v,%v ids=%s,%s", creadoA, creadoB, a.ID, b.ID)
		}
		if p.M.Enviados.Load() != 1 || p.M.Duplicados.Load() != 1 {
			t.Fatal("contadores incorrectos")
		}
	})
}

func TestContrapresionPorCapacidad(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := nuevo(t, Config{Capacidad: 2})
		enviar(t, p, "a", 0)
		enviar(t, p, "b", 0)
		if _, _, err := p.Enviar(modelo.Solicitud{Tipo: "c"}); !errors.Is(err, modelo.ErrColaLlena) {
			t.Fatalf("con la cola llena: %v", err)
		}
		arrendar(t, p, "w", 1) // libera un hueco
		if _, _, err := p.Enviar(modelo.Solicitud{Tipo: "c"}); err != nil {
			t.Fatalf("tras liberar un hueco: %v", err)
		}
	})
}

func TestLimiteDeTasaPorTenant(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := nuevo(t, Config{TasaTenant: 1, RafagaTenant: 2})
		env := func(tenant string) error {
			_, _, err := p.Enviar(modelo.Solicitud{Tipo: "x", Tenant: tenant})
			return err
		}
		if env("a") != nil || env("a") != nil {
			t.Fatal("la ráfaga de 2 debía pasar")
		}
		var lim *modelo.ErrLimite
		if err := env("a"); !errors.As(err, &lim) || lim.RetryAfter <= 0 || lim.RetryAfter > time.Second {
			t.Fatalf("3.º envío: %v", err)
		}
		if err := env("b"); err != nil { // otro tenant: cuota independiente
			t.Fatalf("tenant b: %v", err)
		}
		time.Sleep(time.Second)
		if err := env("a"); err != nil {
			t.Fatalf("tras 1 s debía haber una ficha: %v", err)
		}
		if p.M.RechazadosTasa.Load() != 1 {
			t.Fatalf("rechazados por tasa = %d", p.M.RechazadosTasa.Load())
		}
	})
}

func TestLongPolling(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := nuevo(t, Config{})
		t0 := time.Now()
		got := make(chan []modelo.Job)
		go func() {
			js, _ := p.Arrendar(t.Context(), "w", 5, 10*time.Second)
			got <- js
		}()
		synctest.Wait() // el worker espera
		time.Sleep(3 * time.Second)
		enviar(t, p, "llegó", 0)
		js := <-got
		if len(js) != 1 || time.Since(t0) != 3*time.Second {
			t.Fatalf("jobs=%d tras %v; esperado 1 a los 3 s", len(js), time.Since(t0))
		}
		// sin trabajo: devuelve vacío exactamente al vencer `espera`
		t1 := time.Now()
		js, err := p.Arrendar(t.Context(), "w", 5, 4*time.Second)
		if err != nil || len(js) != 0 || time.Since(t1) != 4*time.Second {
			t.Fatalf("espera vacía: %d jobs, err=%v, %v", len(js), err, time.Since(t1))
		}
	})
}

func TestLongPollingRespetaElContexto(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := nuevo(t, Config{})
		ctx, cancel := context.WithCancel(t.Context())
		res := make(chan error)
		go func() { _, err := p.Arrendar(ctx, "w", 1, time.Hour); res <- err }()
		synctest.Wait()
		cancel()
		if err := <-res; !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v", err)
		}
		if p.esperando != 0 {
			t.Fatalf("esperando = %d tras cancelar", p.esperando)
		}
	})
}

func TestUnTrabajoDespiertaSoloLoNecesario(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := nuevo(t, Config{})
		var obtuvieron atomic.Int32
		var wg sync.WaitGroup
		for i := range 3 {
			wg.Go(func() {
				js, _ := p.Arrendar(t.Context(), "w"+string(rune('0'+i)), 1, 10*time.Second)
				obtuvieron.Add(int32(len(js)))
			})
		}
		synctest.Wait()
		enviar(t, p, "a", 0)
		enviar(t, p, "b", 0)
		wg.Wait()
		if obtuvieron.Load() != 2 {
			t.Fatalf("se arrendaron %d trabajos, esperado 2 (había 3 workers)", obtuvieron.Load())
		}
	})
}

func TestCancelar(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := nuevo(t, Config{})
		a := enviar(t, p, "a", 0)
		b := enviar(t, p, "b", 0)
		if err := p.Cancelar(a.ID); err != nil {
			t.Fatal(err)
		}
		js := arrendar(t, p, "w", 10)
		if len(js) != 1 || js[0].ID != b.ID {
			t.Fatalf("el cancelado no debe arrendarse: %+v", js)
		}
		if err := p.Cancelar(b.ID); !errors.Is(err, modelo.ErrEstadoInvalido) {
			t.Fatalf("cancelar en curso = %v", err)
		}
		if p.Estadisticas().EnEspera != 0 {
			t.Fatal("en_espera debe ser 0")
		}
	})
}

func TestEsperarYPurga(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := nuevo(t, Config{Retencion: time.Minute})
		j, _, _ := p.Enviar(modelo.Solicitud{Tipo: "x", Clave: "k"})
		fin := make(chan modelo.Job)
		go func() { r, _ := p.Esperar(t.Context(), j.ID, time.Hour); fin <- r }()
		synctest.Wait()
		arrendar(t, p, "w", 1)
		time.Sleep(2 * time.Second)
		p.Ack(j.ID, "w", []byte(`42`))
		if r := <-fin; r.Estado != modelo.Completado || string(r.Resultado) != "42" {
			t.Fatalf("Esperar devolvió %+v", r)
		}
		time.Sleep(time.Minute + time.Second)
		synctest.Wait()
		if _, ok := p.Obtener(j.ID); ok {
			t.Fatal("el trabajo terminado debía purgarse tras la retención")
		}
		if _, creado, _ := p.Enviar(modelo.Solicitud{Tipo: "x", Clave: "k"}); !creado {
			t.Fatal("la clave de idempotencia debía liberarse con la purga")
		}
	})
}

func TestEstadisticas(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p := nuevo(t, Config{})
		enviar(t, p, "a", 3)
		enviar(t, p, "b", 1)
		j := enviar(t, p, "c", 1)
		arrendar(t, p, "w", 1) // se lleva "a"
		st := p.Estadisticas()
		if st.Estados[modelo.EnCurso] != 1 || st.Estados[modelo.Pendiente] != 2 || st.PorNivel[1] != 2 || st.EnEspera != 2 {
			t.Fatalf("%+v", st)
		}
		_ = j
	})
}

// Prueba de estrés con el detector de carreras: muchos productores y consumidores reales; cada
// trabajo debe completarse exactamente una vez.
func TestEstresExactamenteUnaVez(t *testing.T) {
	p := Nuevo(Config{Capacidad: 1 << 20})
	ctx, cancel := context.WithCancel(context.Background())
	hecho := make(chan struct{})
	go func() { p.Run(ctx); close(hecho) }()
	defer func() { cancel(); <-hecho }()

	const prod, cada, cons = 8, 1500, 8
	var vistos sync.Map
	var completados atomic.Int64
	var wp, wc sync.WaitGroup
	for i := range prod {
		wp.Go(func() {
			for k := range cada {
				if _, _, err := p.Enviar(modelo.Solicitud{Tipo: "t", Prioridad: (i + k) % modelo.Niveles}); err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	cc, parar := context.WithCancel(ctx)
	for c := range cons {
		wc.Go(func() {
			w := "w" + string(rune('a'+c))
			for cc.Err() == nil {
				js, _ := p.Arrendar(cc, w, 4, 20*time.Millisecond)
				for _, j := range js {
					if _, dup := vistos.LoadOrStore(j.ID, true); dup {
						t.Errorf("trabajo %s entregado dos veces", j.ID)
					}
					if err := p.Ack(j.ID, w, nil); err != nil {
						t.Errorf("Ack: %v", err)
					}
					completados.Add(1)
				}
			}
		})
	}
	wp.Wait()
	for completados.Load() < prod*cada {
		time.Sleep(5 * time.Millisecond)
	}
	parar()
	wc.Wait()
	if st := p.Estadisticas(); st.Estados[modelo.Completado] != prod*cada || st.EnEspera != 0 {
		t.Fatalf("estadísticas finales: %+v", st)
	}
}

func BenchmarkEnviarArrendarAck(b *testing.B) {
	p := Nuevo(Config{Capacidad: 1 << 22})
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			j, _, _ := p.Enviar(modelo.Solicitud{Tipo: "b"})
			js, _ := p.Arrendar(context.Background(), "w", 1, 0)
			for _, x := range js {
				p.Ack(x.ID, "w", nil)
			}
			_ = j
		}
	})
}

func BenchmarkEnviar(b *testing.B) {
	p := Nuevo(Config{Capacidad: 1 << 24})
	b.ResetTimer()
	for range b.N {
		p.Enviar(modelo.Solicitud{Tipo: "b"})
	}
}
