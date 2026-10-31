//go:build !darwin && !linux

package main

const tidDisponible = false

func tid() int { return 0 }
