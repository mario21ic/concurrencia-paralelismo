# Ejercicio 1: reproducir fallos intermitentes con las opciones de `go test`

## Teoría

Un fallo concurrente intermitente se vuelve reproducible combinando opciones:

| Síntoma | Opción | Por qué |
|--|--|--|
| "a veces falla" | `-count=100` | repite hasta que ocurra (y evita la caché de resultados) |
| datos corruptos, contadores erróneos | `-race` | señala el acceso sin sincronizar, con las dos pilas |
| "solo falla en CI / en otra máquina" | `-cpu 1,2,4,8` | cambia `GOMAXPROCS`; algunos bugs necesitan paralelismo real |
| "falla si se ejecuta el test X antes" | `-shuffle=on` | aleatoriza el orden; imprime la semilla |
| se cuelga | `-timeout 30s` | aborta y vuelca todas las pilas |

**Reproducir un fallo de orden:** `-shuffle=on` imprime `-test.shuffle <semilla>`. Repite con `-shuffle=<semilla>` para el mismo orden.

### Estado global y `t.Cleanup`

Un test que modifica estado global (variables de paquete, variables de entorno, el reloj) y no lo restaura contamina a los siguientes. Solución: guardar el valor anterior y restaurarlo con `t.Cleanup(func(){...})` (o `t.Setenv`, `t.Chdir` cuando existan).

### `t.Parallel()`

Marca un (sub)test para ejecutarse **a la vez** que otros paralelos. Reglas:
- Cada subtest paralelo debe usar **su propio estado**; compartir variables entre ellos es una carrera.
- Desde Go 1.22 la variable de bucle es por iteración, así que `for _, tc := range casos { t.Run(..., func(t *testing.T){ t.Parallel(); use(tc) }) }` es seguro.
- `-parallel N` limita cuántos corren a la vez.

## Qué contiene

- `contador.go`: `ContadorMalo` (sin sincronización), `Contador` (correcto) y una variable global `Entorno`.
- `contador_test.go`: tests correctos (siempre en verde).
- `bugs_test.go` (`-tags bugs`): tres tests que fallan por un defecto real.

```bash
go test -race -count=3 -shuffle=on .                              # todo en verde
go test -tags bugs -run TestContadorMalo -count=3 .               # pérdida de incrementos
go test -tags bugs -run TestContadorMalo -cpu 1 -count=5 .        # el bug "desaparece"
go test -tags bugs -race -run TestContadorMalo .                  # el detector lo señala
go test -tags bugs -run 'TestA_|TestB_' -shuffle=on -v .          # depende del orden
```

## Resultados de referencia

```
== contador malo (3 ejecuciones)
Valor = 45925, want 100000 (se perdieron incrementos)
Valor = 44284, want 100000
Valor = 39187, want 100000

== con -cpu 1 (5 repeticiones)
ok                       <- ¡pasa! Con un solo procesador casi no hay interleavings dañinas

== con -race
WARNING: DATA RACE ... ContadorMalo.Inc() ...

== con -shuffle=on (esta ejecución puso A antes que B)
-test.shuffle 1793223976584651000
=== RUN   TestA_CambiaEntornoSinRestaurar
--- PASS
=== RUN   TestB_EsperaEntornoPorDefecto
    Entorno = "test", want produccion (¿otro test lo dejó sucio?)
--- FAIL
```

## Conclusiones

- Con 8 CPUs el contador perdió más de la mitad de los incrementos; con `-cpu 1` **pasó**. Un test verde en una configuración no dice nada de las demás.
- El fallo de orden **pasa en el orden del archivo** (B antes que A) y solo `-shuffle` lo destapa. Sin él, el bug permanece oculto hasta que alguien añade un test y cambia el orden.
- `-race` es la herramienta más directa: no necesita que el resultado salga mal, basta con que el acceso ocurra.
- Arreglo del estado global: `t.Cleanup` (ver `TestEntornoRestaurado`).

## Ejercicios propuestos

1. Haz fallar `TestContadorMalo` con `-cpu 2`: ¿a partir de cuántas CPUs es fiable?
2. Reproduce siempre el mismo fallo de orden con `-shuffle=<semilla>`.
3. Añade un test paralelo que comparta una variable entre subtests y detéctalo con `-race`.
4. Arregla `TestA_` con `t.Cleanup` y verifica con `-shuffle=on -count=50`.
