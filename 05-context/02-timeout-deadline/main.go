// Ejercicio 2: WithTimeout, WithDeadline y peticiones HTTP con contexto.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"
)

func operacion(ctx context.Context, d time.Duration) error {
	select {
	case <-time.After(d):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func main() {
	// 1) WithTimeout: duración relativa.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := operacion(ctx, 300*time.Millisecond)
	fmt.Println("timeout:", err, "| es DeadlineExceeded:", errors.Is(err, context.DeadlineExceeded))

	// 2) WithDeadline: instante absoluto (WithTimeout es WithDeadline(now+d)).
	ctx2, cancel2 := context.WithDeadline(context.Background(), time.Now().Add(50*time.Millisecond))
	defer cancel2()
	d, ok := ctx2.Deadline()
	fmt.Printf("deadline en %v (ok=%v)\n", time.Until(d).Round(10*time.Millisecond), ok)

	// 3) Un hijo no puede ALARGAR el plazo del padre; si pide más, hereda el del padre.
	padre, cp := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cp()
	hijo, ch := context.WithTimeout(padre, 10*time.Second)
	defer ch()
	dp, _ := padre.Deadline()
	dh, _ := hijo.Deadline()
	fmt.Println("hijo hereda el plazo más corto del padre:", dp.Equal(dh))

	// 4) Distinguir Canceled de DeadlineExceeded.
	c3, cancel3 := context.WithCancel(context.Background())
	cancel3()
	fmt.Println("cancelado manualmente:", c3.Err(), "| es Canceled:", errors.Is(c3.Err(), context.Canceled))

	// 5) HTTP: el cliente aborta y el servidor se entera por r.Context().
	servidorSupo := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-time.After(500 * time.Millisecond):
			fmt.Fprint(w, "ok")
		case <-r.Context().Done():
			servidorSupo <- "servidor: el cliente canceló (" + r.Context().Err().Error() + ")"
		}
	}))
	defer srv.Close()

	ctxHTTP, cancelHTTP := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancelHTTP()
	req, _ := http.NewRequestWithContext(ctxHTTP, http.MethodGet, srv.URL, nil)
	ini := time.Now()
	_, err = http.DefaultClient.Do(req)
	fmt.Printf("cliente: %v (tras %v)\n", err, time.Since(ini).Round(10*time.Millisecond))
	fmt.Println(<-servidorSupo)
}
