// Detección de fugas con el perfil EXPERIMENTAL "goroutineleak" de Go 1.26.
//
//	GOEXPERIMENT=goroutineleakprofile go run ./perfil
//
// A diferencia de goleak o del recuento, el runtime usa el RECOLECTOR DE BASURA:
// una goroutine bloqueada en un channel/mutex/etc. que ya NO es alcanzable desde
// ninguna otra goroutine viva nunca podrá despertar => es una fuga segura.
package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/mario21ic/golang-concurrencia-paralelismo/11-testing/05-fugas"
)

func main() {
	perfil := pprof.Lookup("goroutineleak")
	if perfil == nil {
		fmt.Println("el perfil goroutineleak no está disponible.")
		fmt.Println("Ejecuta:  GOEXPERIMENT=goroutineleakprofile go run ./perfil")
		return
	}

	// Dos fugas reales.
	for range 2 {
		fugas.ObtenerMal(func() int { time.Sleep(10 * time.Millisecond); return 1 }, time.Millisecond)
	}
	// Una goroutine bloqueada pero NO fugada: su channel sigue siendo alcanzable.
	vivo := make(chan int)
	go func() { <-vivo }()
	// Una versión correcta: tampoco debe aparecer.
	fugas.ObtenerBien(func() int { time.Sleep(10 * time.Millisecond); return 1 }, time.Millisecond)

	time.Sleep(50 * time.Millisecond) // deja que los trabajos intenten enviar
	runtime.GC()                      // el detector se apoya en el GC

	fmt.Println("== perfil goroutineleak ==")
	perfil.WriteTo(os.Stdout, 1)

	runtime.KeepAlive(vivo) // mantiene `vivo` alcanzable hasta aquí
	fmt.Println("(la goroutine bloqueada en `vivo` NO es una fuga: el channel es alcanzable)")
}
