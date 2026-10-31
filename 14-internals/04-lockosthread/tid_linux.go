//go:build linux

package main

import "syscall"

const tidDisponible = true

func tid() int { return syscall.Gettid() }
