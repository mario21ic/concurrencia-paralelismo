// Ejercicio 4: golang.org/x/sync/singleflight — deduplicar llamadas idénticas.
package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
)

// baseDeDatos simula una consulta costosa y cuenta cuántas veces se ejecuta de verdad.
type baseDeDatos struct{ consultas atomic.Int32 }

func (b *baseDeDatos) Obtener(ctx context.Context, clave string) (string, error) {
	b.consultas.Add(1)
	select {
	case <-time.After(50 * time.Millisecond):
		return "valor de " + clave, nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func lanzar(n int, f func(i int)) {
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() { f(i) })
	}
	wg.Wait()
}

func main() {
	ctx := context.Background()

	// 1) Sin singleflight: 100 peticiones idénticas => 100 consultas.
	var db1 baseDeDatos
	lanzar(100, func(int) { db1.Obtener(ctx, "usuario:42") })
	fmt.Println("1) sin singleflight: consultas a la BD =", db1.consultas.Load())

	// 2) Con singleflight: las concurrentes comparten UNA ejecución.
	var db2 baseDeDatos
	var sf singleflight.Group
	var compartidos atomic.Int32
	lanzar(100, func(int) {
		_, err, shared := sf.Do("usuario:42", func() (any, error) {
			return db2.Obtener(ctx, "usuario:42")
		})
		_ = err
		if shared {
			compartidos.Add(1)
		}
	})
	fmt.Printf("2) con singleflight: consultas a la BD = %d (resultado compartido por %d llamadores)\n", db2.consultas.Load(), compartidos.Load())

	// 3) Claves distintas no se mezclan.
	var db3 baseDeDatos
	var sf3 singleflight.Group
	lanzar(90, func(i int) {
		k := fmt.Sprint("clave", i%3)
		sf3.Do(k, func() (any, error) { return db3.Obtener(ctx, k) })
	})
	fmt.Println("3) 90 llamadas a 3 claves distintas: consultas =", db3.consultas.Load())

	// 4) NO es una caché: solo agrupa llamadas SIMULTÁNEAS.
	var db4 baseDeDatos
	var sf4 singleflight.Group
	for range 3 {
		sf4.Do("k", func() (any, error) { return db4.Obtener(ctx, "k") })
	}
	fmt.Println("4) 3 llamadas secuenciales: consultas =", db4.consultas.Load(), "(no recuerda resultados)")

	// 5) Estampida de caché: expira una clave popular y llegan 200 peticiones.
	cache := struct {
		sync.Mutex
		m map[string]string
	}{m: map[string]string{}}
	var db5 baseDeDatos
	var sf5 singleflight.Group
	leer := func(clave string) string {
		cache.Lock()
		v, ok := cache.m[clave]
		cache.Unlock()
		if ok {
			return v
		}
		r, _, _ := sf5.Do(clave, func() (any, error) {
			val, _ := db5.Obtener(ctx, clave)
			cache.Lock()
			cache.m[clave] = val
			cache.Unlock()
			return val, nil
		})
		return r.(string)
	}
	lanzar(200, func(int) { leer("portada") })
	fmt.Println("5) estampida con caché + singleflight: 200 peticiones => consultas =", db5.consultas.Load())

	// 6) Los errores también se comparten.
	var sf6 singleflight.Group
	var errs atomic.Int32
	lanzar(10, func(int) {
		_, err, _ := sf6.Do("k", func() (any, error) {
			time.Sleep(30 * time.Millisecond)
			return nil, errors.New("BD caída")
		})
		if err != nil {
			errs.Add(1)
		}
	})
	fmt.Println("6) un fallo llegó a", errs.Load(), "de 10 llamadores (todos ven el mismo error)")

	// 7) Trampa de contextos: la ejecución usa el ctx del PRIMER llamador.
	var db7 baseDeDatos
	var sf7 singleflight.Group
	ctxCorto, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel()
	var wg sync.WaitGroup
	var errPrimero, errSegundo error
	wg.Go(func() { // el primero llega con un plazo corto: su ctx manda
		_, errPrimero, _ = sf7.Do("k", func() (any, error) { return db7.Obtener(ctxCorto, "k") })
	})
	time.Sleep(2 * time.Millisecond)
	wg.Go(func() { // el segundo tiene tiempo de sobra, pero comparte el resultado del primero
		_, errSegundo, _ = sf7.Do("k", func() (any, error) { return db7.Obtener(ctx, "k") })
	})
	wg.Wait()
	fmt.Printf("7) trampa del contexto: primero=%v | segundo=%v\n", errPrimero, errSegundo)

	// 8) Solución: ejecutar con un contexto desacoplado y esperar con DoChan + select en cada llamador.
	var db8 baseDeDatos
	var sf8 singleflight.Group
	obtener := func(ctx context.Context, clave string) (string, error) {
		ch := sf8.DoChan(clave, func() (any, error) {
			return db8.Obtener(context.WithoutCancel(ctx), clave) // la ejecución no depende de un solo llamador
		})
		select {
		case r := <-ch:
			if r.Err != nil {
				return "", r.Err
			}
			return r.Val.(string), nil
		case <-ctx.Done(): // cada llamador abandona SU espera sin afectar a los demás
			return "", ctx.Err()
		}
	}
	var e1, e2 error
	ctxImpaciente, cancel2 := context.WithTimeout(ctx, 10*time.Millisecond)
	defer cancel2()
	wg.Go(func() { _, e1 = obtener(ctxImpaciente, "k") })
	time.Sleep(2 * time.Millisecond)
	wg.Go(func() { _, e2 = obtener(ctx, "k") })
	wg.Wait()
	fmt.Printf("8) con DoChan + WithoutCancel: impaciente=%v | paciente=%v | consultas=%d\n", e1, e2, db8.consultas.Load())
}
