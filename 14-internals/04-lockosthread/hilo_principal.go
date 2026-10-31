package main

import "runtime"

// Muchas bibliotecas de interfaz gráfica y gráficos (Cocoa/AppKit en macOS, GLFW, OpenGL,
// SDL) exigen que ciertas llamadas se hagan SIEMPRE desde el hilo principal del proceso.
// Receta estándar:
//
//  1. init() fija la goroutine principal al hilo principal con LockOSThread.
//  2. main() ejecuta un bucle que atiende peticiones.
//  3. Las demás goroutines envían sus llamadas a ese bucle.
func init() {
	runtime.LockOSThread() // los init se ejecutan en la goroutine principal: queda fijada al hilo principal
}

type trabajo struct {
	f     func()
	hecho chan struct{}
}

var cola = make(chan trabajo)

// EnHiloPrincipal ejecuta f en el hilo principal y espera a que termine.
func EnHiloPrincipal(f func()) {
	t := trabajo{f: f, hecho: make(chan struct{})}
	cola <- t
	<-t.hecho
}

// BucleDelHiloPrincipal debe llamarse desde main() (la goroutine fijada al hilo principal).
func BucleDelHiloPrincipal(parar <-chan struct{}) {
	for {
		select {
		case t := <-cola:
			t.f()
			close(t.hecho)
		case <-parar:
			return
		}
	}
}
