# Ejercicio 2: *false sharing* — la contención invisible

## Teoría

La CPU no lee ni escribe bytes sueltos en memoria: mueve **líneas de caché** (64 bytes en la mayoría de x86; **128 bytes en Apple Silicon**; 64/128 en ARM de servidor). El protocolo de coherencia garantiza que una línea esté en modo escritura en **un solo núcleo** a la vez.

**False sharing**: dos goroutines escriben en variables **distintas e independientes** que están **en la misma línea**. Lógicamente no comparten nada, pero físicamente sí: cada escritura invalida la línea en la caché del otro núcleo, y la línea "rebota" entre núcleos. No hay locks, no hay data race, el resultado es correcto… y es mucho más lento.

```
línea de caché (128 B):  [ contador A | contador B | contador C | ... ]
núcleo 0 escribe A  →  invalida la línea en los demás
núcleo 1 escribe B  →  invalida la línea en el núcleo 0 …    (la línea viaja sin parar)
```

### Cómo reconocerlo

- Código que **no escala** (o empeora) al añadir núcleos, sin locks aparentes.
- Contadores/estadísticas **por goroutine** o **por worker** guardados en un array/slice contiguo.
- Estructuras con campos "calientes" escritos por goroutines distintas.
- Perfil de CPU sin culpable claro; herramientas de bajo nivel (`perf c2c` en Linux) lo muestran.

### Cómo se evita

1. **Relleno (*padding*)** hasta separar los datos al menos una línea: `golang.org/x/sys/cpu.CacheLinePad` (usa el tamaño de línea de la arquitectura de compilación; 128 B en arm64).
2. **Agrupar por escritor:** los datos escritos por la misma goroutine juntos; los de goroutines distintas, separados.
3. **Estado local** por goroutine (variables locales) y combinar al final: ni siquiera hay línea compartida.
4. Cuidado: el relleno **gasta memoria** (un contador de 8 B pasa a ocupar 128 B). Úsalo solo en datos calientes y pocos.

### Detalles de Go

- `unsafe.Sizeof`/`unsafe.Offsetof` permiten verificar el diseño real de una estructura.
- El asignador no garantiza alineación a línea de caché; el programa alinea explícitamente su búfer.
- `sync.Pool`, `sync.Map` y los canales ya cuidan estos detalles internamente.

## Qué hace el programa

1. 8 goroutines, cada una incrementa **su propio** contador 150 M veces. Los contadores se separan 8, 16, 32, 64, 128 y 256 bytes.
2. El caso "natural": una estructura con dos campos escritos por dos goroutines, sin y con `CacheLinePad`.

```bash
go run .
go test -race .
```

## Resultados de referencia (M1, línea de caché de 128 B)

```
cpu.CacheLinePad = 128 bytes

 separación       tiempo   vs. la mejor
        8 B        834ms           3.4x
       16 B        739ms           3.0x
       32 B        519ms           2.1x
       64 B        339ms           1.4x       <- ¡64 B NO basta en este chip!
      128 B        283ms           1.1x
      256 B        246ms           1.0x

dos campos en una estructura:
juntos   ( 16 bytes): 147ms
separados(144 bytes): 90ms    (1.6× más rápido)
```

## Conclusiones

- Con los contadores pegados (8 B), el programa tarda **3.4× más** que con ellos separados, sin ningún lock y con resultados correctos.
- El efecto desaparece de forma **gradual** al aumentar la separación: cuántos contadores comparten línea (16 con 8 B, 8 con 16 B, 2 con 64 B, 1 con ≥128 B).
- **El "64 bytes" de los manuales no es universal.** En este chip las líneas son de 128 B: separar 64 B deja pares de contadores en la misma línea (aún 1.4× más lento). Por eso el código usa `cpu.CacheLinePad` en vez de un `[64]byte` fijo.
- La diferencia entre 128 B (1.1×) y 256 B (1.0×) probablemente es el prefetcher de líneas adyacentes o ruido; no se investigó.
- Un caso realista (dos campos de estadísticas) pierde ~1.6×; con más escritores, más.
- La corrección es barata en código (una línea de relleno) pero hay que **saber dónde**: mide antes.

## Tests

Cada contador cuenta lo suyo para saltos de 8, 64 y 128 bytes; el búfer queda alineado a 256; la estructura separada realmente separa los campos al menos una línea.

## Ejercicios propuestos

1. Cambia la estructura para que un campo lo escriban 4 goroutines y otro solo 1: ¿qué agrupación conviene?
2. Reemplaza `*p++` por `atomic.AddUint64(p, 1)` y compara: ¿el atómico cambia el efecto?
3. Mide con 2, 4 y 8 goroutines: ¿cómo crece la penalización?
4. Revisa tu propio código: ¿hay arrays de contadores/estadísticas por worker?
