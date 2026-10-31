// Ejercicio 4: WithCancelCause, WithTimeoutCause, AfterFunc y WithoutCancel.
package main

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type claveUsuario struct{}

var errCuotaExcedida = errors.New("cuota excedida")

func main() {
	// 1) WithCancelCause: ctx.Err() solo dice "canceled"; Cause dice POR QUÉ.
	ctx, cancel := context.WithCancelCause(context.Background())
	cancel(errCuotaExcedida)
	fmt.Println("Err:   ", ctx.Err())
	fmt.Println("Cause: ", context.Cause(ctx), "| es errCuotaExcedida:", errors.Is(context.Cause(ctx), errCuotaExcedida))

	// 2) WithTimeoutCause: causa personalizada cuando vence el plazo.
	ctx2, cancel2 := context.WithTimeoutCause(context.Background(), 30*time.Millisecond, errors.New("la base de datos no respondió"))
	defer cancel2()
	<-ctx2.Done()
	fmt.Println("Err:   ", ctx2.Err())
	fmt.Println("Cause: ", context.Cause(ctx2))

	// 3) AfterFunc: ejecuta f en su PROPIA goroutine cuando el contexto termina.
	ctx3, cancel3 := context.WithCancel(context.Background())
	limpiado := make(chan struct{})
	stop := context.AfterFunc(ctx3, func() {
		fmt.Println("AfterFunc: liberando recursos...")
		close(limpiado)
	})
	cancel3()
	<-limpiado
	fmt.Println("stop() tras ejecutarse devuelve:", stop(), "(false = ya corrió)")

	// stop antes de que ocurra la cancelación evita que f corra.
	ctx4, cancel4 := context.WithCancel(context.Background())
	stop4 := context.AfterFunc(ctx4, func() { fmt.Println("¡no debería imprimirse!") })
	fmt.Println("stop() antes de cancelar devuelve:", stop4(), "(true = se evitó)")
	cancel4()
	time.Sleep(20 * time.Millisecond)

	// 4) WithoutCancel: conserva los VALORES pero ignora la cancelación del padre.
	//    Útil para trabajo que debe terminar aunque la petición ya se canceló
	//    (auditoría, limpieza).
	peticion, cancelPeticion := context.WithCancel(context.WithValue(context.Background(), claveUsuario{}, "ana"))
	cancelPeticion()
	auditoria := context.WithoutCancel(peticion)
	fmt.Println("petición cancelada:", peticion.Err())
	fmt.Println("auditoría: Err =", auditoria.Err(), "| usuario =", auditoria.Value(claveUsuario{}))
}
