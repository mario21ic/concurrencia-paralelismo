// Ejercicio 7: publicador/suscriptor.
package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

func main() {
	b := NuevoBroker[int]()
	ctx := context.Background()

	rapido := b.Suscribir(ctx, 100)
	lento := b.Suscribir(ctx, 2)
	ctxCorto, cancelCorto := context.WithCancel(ctx)
	efimero := b.Suscribir(ctxCorto, 100)

	var wg sync.WaitGroup
	var nRapido, nLento, nEfimero int

	wg.Go(func() {
		for range rapido.C {
			nRapido++
		}
	})
	wg.Go(func() {
		for range lento.C {
			nLento++
			time.Sleep(25 * time.Millisecond) // consumidor lento
		}
	})
	wg.Go(func() {
		for range efimero.C {
			nEfimero++
		}
	})

	fmt.Println("suscriptores iniciales:", b.Suscriptores())
	for i := 1; i <= 20; i++ {
		b.Publicar(i)
		time.Sleep(5 * time.Millisecond)
		if i == 8 {
			cancelCorto() // el efímero se da de baja a mitad
			time.Sleep(10 * time.Millisecond)
			fmt.Println("tras cancelar el contexto del efímero:", b.Suscriptores(), "suscriptores")
		}
	}
	b.Cerrar()
	wg.Wait()

	fmt.Printf("\nrápido:  recibió %2d, descartó %d\n", nRapido, rapido.Descartados())
	fmt.Printf("lento:   recibió %2d, descartó %d  <- no frenó a nadie\n", nLento, lento.Descartados())
	fmt.Printf("efímero: recibió %2d, descartó %d  <- se dio de baja\n", nEfimero, efimero.Descartados())
	fmt.Println("publicar tras Cerrar entrega a:", b.Publicar(99))
}
