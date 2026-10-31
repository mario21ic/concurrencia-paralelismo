// Ejercicio 3: sync.Once, OnceFunc, OnceValue, OnceValues.
package main

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

type Config struct{ DSN string }

var llamadasCarga atomic.Int32

func cargarConfig() Config {
	llamadasCarga.Add(1)
	fmt.Println("  (cargando configuración...)")
	return Config{DSN: "postgres://localhost/app"}
}

// Singleton perezoso, seguro entre goroutines, en una sola línea.
var config = sync.OnceValue(cargarConfig)

var intentosBD atomic.Int32

// OnceValues cachea (valor, error): si falla, el error también queda cacheado.
var conexion = sync.OnceValues(func() (string, error) {
	intentosBD.Add(1)
	return "", errors.New("base de datos no disponible")
})

func main() {
	// 1) Once clásico: la función corre una sola vez aunque 10 goroutines lo pidan.
	var once sync.Once
	var cuenta int
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() { once.Do(func() { cuenta++ }) })
	}
	wg.Wait()
	fmt.Println("Once.Do ejecutó f", cuenta, "vez (de 10 llamadas)")

	// 2) OnceValue: perezoso y cacheado.
	fmt.Println("antes de usar config, cargas:", llamadasCarga.Load())
	for range 3 {
		wg.Go(func() { _ = config() })
	}
	wg.Wait()
	fmt.Println("config:", config(), "| cargas:", llamadasCarga.Load())

	// 3) OnceValues: el error también se cachea; no hay reintento.
	for range 3 {
		_, err := conexion()
		fmt.Println("conexion():", err)
	}
	fmt.Println("intentos reales de conexión:", intentosBD.Load(), "(para reintentar, usa un mutex propio)")

	// 4) Si f entra en pánico, Once lo da por hecho: no se vuelve a ejecutar.
	var once2 sync.Once
	func() {
		defer func() { fmt.Println("Once.Do con panic recuperado:", recover()) }()
		once2.Do(func() { panic("falló la inicialización") })
	}()
	ejecutada := false
	once2.Do(func() { ejecutada = true })
	fmt.Println("¿segundo Do ejecutó f?", ejecutada)

	// 5) OnceFunc: si f entra en pánico, CADA llamada vuelve a lanzar el mismo pánico.
	f := sync.OnceFunc(func() { panic("boom") })
	for i := 1; i <= 2; i++ {
		func() {
			defer func() { fmt.Printf("OnceFunc llamada %d -> panic: %v\n", i, recover()) }()
			f()
		}()
	}
}
