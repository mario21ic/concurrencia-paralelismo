//go:build darwin

package main

import "syscall"

const tidDisponible = true

// tid devuelve el identificador del HILO DEL SO que ejecuta la goroutine ahora mismo.
func tid() int {
	const sysThreadSelfID = 372 // SYS_THREAD_SELFID en macOS (ver golang.org/x/sys/unix)
	r, _, _ := syscall.RawSyscall(sysThreadSelfID, 0, 0, 0)
	return int(r)
}
