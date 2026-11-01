package main

import (
	"testing"
	"testing/synctest"
)

func TestSumaDeCuadrados(t *testing.T) {
	if got := Sumar(Productor(10)); got != 385 {
		t.Fatalf("suma = %d, esperado 385", got)
	}
}

func TestElProductorCierraElChannel(t *testing.T) {
	ch := Productor(3)
	for range 3 {
		<-ch
	}
	if _, abierto := <-ch; abierto {
		t.Fatal("el channel debía estar cerrado tras el último valor")
	}
}

func TestElBufferDejaAdelantarseAlProductor(t *testing.T) {
	for buffer, esperado := range map[int]int{0: 1, 1: 2, 4: 5} {
		synctest.Test(t, func(t *testing.T) {
			// El consumidor se lleva 1 y trabaja en él; el productor puede dejar `buffer` más en el
			// canal y, si no hay buffer, queda bloqueado en el 2.º envío.
			if got := Adelanto(buffer); got != esperado {
				t.Errorf("buffer=%d: adelanto=%d, esperado %d", buffer, got, esperado)
			}
		})
	}
}
