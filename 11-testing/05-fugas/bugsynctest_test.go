//go:build bugsynctest

package fugas

import (
	"testing"
	"testing/synctest"
	"time"
)

// synctest no hace fallar el test: ENTRA EN PÁNICO y aborta todo el binario de
// tests. Por eso va en un archivo (y una etiqueta) aparte.
func TestObtenerMal_Synctest(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ObtenerMal(func() int { time.Sleep(time.Hour); return 1 }, time.Second)
		time.Sleep(time.Hour) // incluso dejando terminar el trabajo, queda bloqueada en `ch <-`
	})
}
