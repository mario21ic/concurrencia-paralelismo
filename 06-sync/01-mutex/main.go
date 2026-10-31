// Ejercicio 1: sync.Mutex — proteger invariantes.
//
//	go run .                  # transferencias correctas (locks ordenados)
//	go run . -modo=deadlock   # orden inconsistente de locks => deadlock
package main

import (
	"errors"
	"flag"
	"fmt"
	"sync"
	"time"
)

var ErrSaldoInsuficiente = errors.New("saldo insuficiente")

// Cuenta protege su invariante (saldo >= 0). El mutex va junto a lo que protege
// y el campo no se exporta: solo se toca a través de métodos.
type Cuenta struct {
	id    int
	mu    sync.Mutex
	saldo int
}

func (c *Cuenta) Depositar(n int) {
	c.mu.Lock()
	defer c.mu.Unlock() // defer: se libera aunque haya return o panic
	c.saldo += n
}

func (c *Cuenta) Retirar(n int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.saldo < n {
		return ErrSaldoInsuficiente
	}
	c.saldo -= n
	return nil
}

func (c *Cuenta) Saldo() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.saldo
}

// Transferir toma DOS locks. Para evitar deadlock, todas las goroutines
// deben tomarlos en el MISMO orden global (aquí, por id).
func Transferir(origen, destino *Cuenta, n int) error {
	if origen == destino {
		return nil
	}
	primero, segundo := origen, destino
	if primero.id > segundo.id {
		primero, segundo = segundo, primero
	}
	primero.mu.Lock()
	defer primero.mu.Unlock()
	segundo.mu.Lock()
	defer segundo.mu.Unlock()

	if origen.saldo < n {
		return ErrSaldoInsuficiente
	}
	origen.saldo -= n
	destino.saldo += n
	return nil
}

// TransferirMal toma los locks en el orden "origen, destino": A→B y B→A
// a la vez se bloquean mutuamente.
func TransferirMal(origen, destino *Cuenta, n int) {
	origen.mu.Lock()
	defer origen.mu.Unlock()
	time.Sleep(10 * time.Millisecond) // garantiza que ambas tengan su primer lock
	destino.mu.Lock()
	defer destino.mu.Unlock()
	origen.saldo -= n
	destino.saldo += n
}

func main() {
	modo := flag.String("modo", "ok", "ok|deadlock")
	flag.Parse()

	a, b := &Cuenta{id: 1, saldo: 1000}, &Cuenta{id: 2, saldo: 1000}

	if *modo == "deadlock" {
		var wg sync.WaitGroup
		wg.Go(func() { TransferirMal(a, b, 10) })
		wg.Go(func() { TransferirMal(b, a, 10) })
		wg.Wait()
		return
	}

	var wg sync.WaitGroup
	var fallidas, intentos = 0, 0
	var mu sync.Mutex
	for i := range 1000 {
		wg.Go(func() {
			var err error
			if i%2 == 0 {
				err = Transferir(a, b, 7)
			} else {
				err = Transferir(b, a, 5)
			}
			mu.Lock()
			intentos++
			if err != nil {
				fallidas++
			}
			mu.Unlock()
		})
	}
	wg.Wait()
	fmt.Printf("a=%d b=%d total=%d (esperado 2000)\n", a.Saldo(), b.Saldo(), a.Saldo()+b.Saldo())
	fmt.Printf("transferencias: %d, rechazadas: %d\n", intentos, fallidas)

	// TryLock: intenta tomar el lock sin bloquear (úsalo con moderación).
	var m sync.Mutex
	m.Lock()
	fmt.Println("TryLock con el lock tomado:", m.TryLock())
	m.Unlock()
	fmt.Println("TryLock con el lock libre: ", m.TryLock())
	m.Unlock()
}
