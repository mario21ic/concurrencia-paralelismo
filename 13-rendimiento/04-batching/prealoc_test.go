package main

import (
	"strings"
	"testing"
)

// ---- Reducir asignaciones: preasignar cuando se conoce el tamaño ----

const n = 1000

var sink []int

func BenchmarkAppendSinPreasignar(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		var s []int
		for i := range n {
			s = append(s, i)
		}
		sink = s
	}
}

func BenchmarkAppendPreasignado(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		s := make([]int, 0, n)
		for i := range n {
			s = append(s, i)
		}
		sink = s
	}
}

var sinkMap map[int]int

func BenchmarkMapSinPreasignar(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		m := map[int]int{}
		for i := range n {
			m[i] = i
		}
		sinkMap = m
	}
}

func BenchmarkMapPreasignado(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		m := make(map[int]int, n)
		for i := range n {
			m[i] = i
		}
		sinkMap = m
	}
}

var sinkStr string

func BenchmarkStringConcatenar(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		s := ""
		for range 100 {
			s += "abc"
		}
		sinkStr = s
	}
}

func BenchmarkStringBuilderPreasignado(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		var sb strings.Builder
		sb.Grow(300)
		for range 100 {
			sb.WriteString("abc")
		}
		sinkStr = sb.String()
	}
}
