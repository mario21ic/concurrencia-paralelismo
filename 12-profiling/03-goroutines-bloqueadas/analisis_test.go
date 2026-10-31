package main

import "testing"

const muestraDebug1 = `goroutine profile: total 6
3 @ 0x1 0x2 0x3
#	0x100	main.fuga.func1+0x20	/app/main.go:30

2 @ 0x1 0x2
#	0x200	runtime.gopark+0x10	/go/src/runtime/proc.go:400
#	0x201	main.otra+0x30	/app/otra.go:5

1 @ 0x9
#	0x300	net/http.(*conn).serve+0x40	/go/src/net/http/server.go:2000
`

func TestAgruparPilas(t *testing.T) {
	g := AgruparPilas(muestraDebug1)
	if len(g) != 3 || g[0].N != 3 || g[0].Funcion != "main.fuga.func1" || g[0].Origen != "/app/main.go:30" {
		t.Fatalf("grupos = %+v", g)
	}
	if g[1].N != 2 || g[2].N != 1 {
		t.Fatalf("orden incorrecto: %+v", g)
	}
}

func TestContarPorEstado(t *testing.T) {
	salida := `goroutine 1 [running]:
main.main()

goroutine 7 [chan receive, 2 minutes]:
main.f()

goroutine 8 [chan receive]:
main.f()

goroutine 9 [sync.Mutex.Lock, 1 minutes]:
main.g()
`
	c := ContarPorEstado(salida)
	if c["chan receive"] != 2 || c["running"] != 1 || c["sync.Mutex.Lock"] != 1 {
		t.Fatalf("cuenta = %v", c)
	}
}
