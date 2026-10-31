// Ejercicio 5: context.WithValue — cómo y cuándo (poco).
package main

import (
	"context"
	"fmt"
)

// Clave de tipo propio NO exportado: evita colisiones entre paquetes.
type claveCtx int

const (
	claveIDPeticion claveCtx = iota
	claveUsuario
)

// Funciones de acceso tipadas: el resto del código no ve las claves.
func ConIDPeticion(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, claveIDPeticion, id)
}

func IDPeticion(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(claveIDPeticion).(string)
	return id, ok
}

func ConUsuario(ctx context.Context, u string) context.Context {
	return context.WithValue(ctx, claveUsuario, u)
}

func Usuario(ctx context.Context) string {
	u, _ := ctx.Value(claveUsuario).(string)
	return u
}

func capaProfunda(ctx context.Context) {
	id, _ := IDPeticion(ctx)
	fmt.Printf("  [capaProfunda] id=%q usuario=%q\n", id, Usuario(ctx))
}

func capaMedia(ctx context.Context) {
	capaProfunda(ctx) // el valor viaja sin ser parámetro explícito
}

func main() {
	ctx := ConIDPeticion(context.Background(), "req-42")
	ctx = ConUsuario(ctx, "ana")
	fmt.Println("valores propagados por la cadena de llamadas:")
	capaMedia(ctx)

	// Un contexto hijo puede sombrear el valor del padre sin modificarlo.
	hijo := ConUsuario(ctx, "admin")
	fmt.Println("hijo (usuario sombreado):")
	capaProfunda(hijo)
	fmt.Println("padre intacto:")
	capaProfunda(ctx)

	// Con claves string hay colisión silenciosa entre paquetes:
	a := context.WithValue(context.Background(), "usuario", "del paquete A")
	b := context.WithValue(a, "usuario", "del paquete B") // pisa sin avisar
	fmt.Println("claves string colisionan:", b.Value("usuario"))

	// Con tipos distintos, no.
	type claveA struct{}
	type claveB struct{}
	c := context.WithValue(context.Background(), claveA{}, "A")
	c = context.WithValue(c, claveB{}, "B")
	fmt.Println("claves de tipos distintos:", c.Value(claveA{}), c.Value(claveB{}))

	// Búsqueda de una clave inexistente => nil.
	fmt.Println("clave ausente:", c.Value("no existe"))
}
