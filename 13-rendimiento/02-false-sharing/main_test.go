package main

import (
	"testing"
	"unsafe"

	"golang.org/x/sys/cpu"
)

func TestCadaContadorCuentaLoSuyo(t *testing.T) {
	for _, salto := range []uintptr{8, 64, 128} {
		c := Nuevo(8, salto)
		c.Correr(8, 10_000)
		for i := range 8 {
			if *c.ptr(i) != 10_000 {
				t.Fatalf("salto=%d: contador %d = %d, want 10000", salto, i, *c.ptr(i))
			}
		}
	}
}

func TestAlineacion(t *testing.T) {
	c := Nuevo(4, 128)
	if uintptr(c.base)%256 != 0 {
		t.Fatalf("base no alineada a 256: %x", uintptr(c.base))
	}
}

func TestEstructuraSeparadaTienePadding(t *testing.T) {
	var s EstadisticasSeparadas
	if d := unsafe.Offsetof(s.Errores) - unsafe.Offsetof(s.Peticiones); d < unsafe.Sizeof(cpu.CacheLinePad{}) {
		t.Fatalf("los campos están a %d bytes; se esperaban al menos %d", d, unsafe.Sizeof(cpu.CacheLinePad{}))
	}
}
