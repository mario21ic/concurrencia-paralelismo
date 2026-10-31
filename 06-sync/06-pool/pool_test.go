package main

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
)

var salida string

func BenchmarkSinPool(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			buf := new(bytes.Buffer)
			fmt.Fprintf(buf, "usuario-%d", i)
			salida = buf.String()
			i++
		}
	})
}

func BenchmarkConPool(b *testing.B) {
	b.ReportAllocs()
	p := sync.Pool{New: func() any { return new(bytes.Buffer) }}
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			buf := p.Get().(*bytes.Buffer)
			buf.Reset()
			fmt.Fprintf(buf, "usuario-%d", i)
			salida = buf.String()
			p.Put(buf)
			i++
		}
	})
}

// Buffers grandes: aquí el pool sí marca la diferencia.
func BenchmarkGrandeSinPool(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := make([]byte, 64*1024)
			buf[0] = 1
			_ = buf
			sinkBytes = buf
		}
	})
}

func BenchmarkGrandeConPool(b *testing.B) {
	b.ReportAllocs()
	p := sync.Pool{New: func() any { s := make([]byte, 64*1024); return &s }}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			bp := p.Get().(*[]byte)
			(*bp)[0] = 1
			p.Put(bp) // puntero a slice: Put de un slice directo asigna al convertir a any
		}
	})
}

var sinkBytes []byte
