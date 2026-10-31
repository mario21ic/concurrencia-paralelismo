// Package flags reúne código con defectos de concurrencia típicos para
// practicar las opciones de `go test`.
package flags

import "sync"

// ContadorMalo incrementa sin sincronización: data race.
type ContadorMalo struct{ n int }

func (c *ContadorMalo) Inc()       { c.n++ }
func (c *ContadorMalo) Valor() int { return c.n }

// Contador es la versión correcta.
type Contador struct {
	mu sync.Mutex
	n  int
}

func (c *Contador) Inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.n++
}

func (c *Contador) Valor() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

// Entorno es estado global mutable: la fuente típica de tests que dependen
// del orden de ejecución.
var Entorno = "produccion"
