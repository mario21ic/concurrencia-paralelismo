# Ejercicio 1: operaciones atómicas y tipos atómicos

## Teoría

Una operación atómica es **indivisible** para el resto de goroutines: nadie observa un estado intermedio. Cuesta una instrucción de CPU especial (`LOCK XADD`, `LDADD`, `CAS`…), más barata que un mutex, pero no gratis: fuerza la sincronización de cachés entre núcleos.

| Método | Qué hace | Retorno |
|--|--|--|
| `Add(d)` | suma `d` | valor **nuevo** |
| `Swap(v)` | sustituye | valor **anterior** |
| `CompareAndSwap(old, new)` | cambia solo si vale `old` | `true` si cambió |
| `Load()` / `Store(v)` | lectura / escritura | |

### Tipos vs funciones

```go
var n atomic.Int64     // moderno: n.Add(1), n.Load()
var m int64            // antiguo: atomic.AddInt64(&m, 1)
```

Prefiere los tipos: no puedes leer el valor sin atomic por error, la alineación a 64 bits está garantizada y no se pueden copiar (`go vet`).

### Restar en tipos sin signo

No hay `Sub`. Se suma el complemento a dos: `u.Add(^uint64(0))` resta 1; `u.Add(^uint64(n-1))` resta `n`.

### Banderas con `atomic.Bool`

Para señalar "parar" entre goroutines, `Bool` es suficiente (más simple que un channel si solo se consulta periódicamente). Una `bool` normal sería un data race y el compilador podría incluso sacar la lectura del bucle y no volver a mirarla jamás.

### `CompareAndSwap` como "solo una vez"

`flag.CompareAndSwap(false, true)` lo gana exactamente **una** goroutine. Es la base de `sync.Once`, de elecciones de líder locales y de "el primero que llega inicializa".

## Qué hace el programa

1. Contador compartido por 100 goroutines con `atomic.Int64`.
2. Muestra los valores de retorno de `Add`, `Swap` y `CompareAndSwap`.
3. Compara la forma antigua y la nueva.
4. Resta con `Uint64`.
5. Una bandera `atomic.Bool` detiene a un worker.
6. 50 goroutines compiten por `CompareAndSwap(false, true)`.

```bash
go run .
go run -race .
```

## Resultado de referencia

```
atomic.Int64: 1000000 (esperado 1000000)

Add(5)  devuelve el NUEVO valor:     5
Swap(9) devuelve el valor ANTERIOR:  5
CompareAndSwap(9, 20) =>             true (valor ahora 20 )
CompareAndSwap(9, 30) =>             false (ya no era 9; sigue 20 )
Uint64: 10 - 1 =                     9
50 goroutines compitiendo, ganadoras: 1
```

## Conclusiones

- Exactitud sin locks para un único valor.
- Distingue el valor de retorno de cada método: es una fuente típica de errores.
- Solo una goroutine gana el `CompareAndSwap(false, true)`.

## Ejercicios propuestos

1. Quita atomic de un contador y ejecuta con `-race`.
2. Implementa un contador de "peticiones en vuelo" (`Add(1)` al entrar, `Add(-1)` al salir) con un tope máximo mediante CAS.
3. Usa `atomic.Int32` como estado de una máquina de estados simple (0=iniciando, 1=activo, 2=cerrando) con transiciones por CAS.
