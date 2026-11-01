// Caso 2.2 (intermedio): una caché que se lee mucho, se escribe poco y evita la "estampida".
//
// Problema: 100 peticiones piden a la vez el MISMO dato que no está en caché. Sin cuidado, las 100
// van a la base de datos (estampida de caché / thundering herd). Queremos UNA consulta y que las otras
// 99 esperen su resultado.
//
// Herramientas:
//
//   - sync.RWMutex: muchos lectores a la vez, un escritor en exclusiva (la caché se lee mucho).
//
//   - singleflight.Group: unifica las cargas simultáneas de una misma clave.
//
//     go run .
package main

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/singleflight"
)

type entrada struct {
	valor  string
	caduca time.Time
}

// Cache guarda valores con caducidad (TTL).
type Cache struct {
	ttl   time.Duration
	mu    sync.RWMutex
	datos map[string]entrada
	unico singleflight.Group
}

func NuevaCache(ttl time.Duration) *Cache {
	return &Cache{ttl: ttl, datos: map[string]entrada{}}
}

func (c *Cache) leer(clave string) (string, bool) {
	c.mu.RLock() // varios lectores a la vez
	defer c.mu.RUnlock()
	e, ok := c.datos[clave]
	if !ok || !time.Now().Before(e.caduca) {
		return "", false
	}
	return e.valor, true
}

// Obtener devuelve el valor de la caché o lo carga UNA sola vez aunque lo pidan muchos a la vez.
// Los errores no se cachean (el siguiente intento vuelve a probar).
func (c *Cache) Obtener(clave string, cargar func() (string, error)) (string, error) {
	if v, ok := c.leer(clave); ok {
		return v, nil
	}
	v, err, _ := c.unico.Do(clave, func() (any, error) {
		if v, ok := c.leer(clave); ok { // otra goroutine pudo cargarla mientras esperábamos
			return v, nil
		}
		v, err := cargar()
		if err != nil {
			return "", err
		}
		c.mu.Lock()
		c.datos[clave] = entrada{v, time.Now().Add(c.ttl)}
		c.mu.Unlock()
		return v, nil
	})
	if err != nil {
		return "", err
	}
	return v.(string), nil
}

func main() {
	var consultas atomic.Int32
	bd := func() (string, error) {
		consultas.Add(1)
		time.Sleep(100 * time.Millisecond) // consulta lenta
		return "perfil de ana", nil
	}

	cache := NuevaCache(time.Minute)
	var wg sync.WaitGroup
	t0 := time.Now()
	for range 100 {
		wg.Go(func() { cache.Obtener("usuario:1", bd) })
	}
	wg.Wait()
	fmt.Printf("100 peticiones simultáneas → %d consulta(s) a la BD, en %v\n", consultas.Load(), time.Since(t0).Round(10*time.Millisecond))

	t0 = time.Now()
	cache.Obtener("usuario:1", bd)
	fmt.Printf("petición siguiente (en caché) → %v, consultas totales: %d\n", time.Since(t0), consultas.Load())
}
