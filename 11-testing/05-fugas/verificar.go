package fugas

import (
	"bytes"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"
)

// SinFugas comprueba al final del test que no quedaron goroutines nuevas.
// Es la versión casera de goleak: cuenta goroutines y, si sobran, muestra sus pilas.
//
// Limitaciones: si otro código crea o termina goroutines a la vez, el recuento
// puede confundirse; goleak compara pilas y filtra las conocidas.
func SinFugas(t *testing.T) {
	t.Helper()
	antes := runtime.NumGoroutine()
	t.Cleanup(func() {
		limite := time.Now().Add(time.Second)
		for runtime.NumGoroutine() > antes && time.Now().Before(limite) {
			time.Sleep(5 * time.Millisecond) // dar tiempo a que terminen las que están saliendo
		}
		if despues := runtime.NumGoroutine(); despues > antes {
			var b bytes.Buffer
			pprof.Lookup("goroutine").WriteTo(&b, 1)
			t.Errorf("fuga de goroutines: antes=%d después=%d\n%s", antes, despues, b.String())
		}
	})
}
