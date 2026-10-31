# Ejercicio 6: cómo se implementan los channels y `select`

## Teoría

### `hchan`: la estructura de un channel

`make(chan T, n)` llama a `runtime.makechan` y reserva un `hchan`:

| Campo | Para qué |
|--|--|
| `qcount`, `dataqsiz` | elementos en el buffer / capacidad |
| `buf`, `sendx`, `recvx` | **buffer circular** y los índices de escritura/lectura |
| `closed` | si se cerró |
| `recvq`, `sendq` | colas **FIFO** de goroutines bloqueadas (cada una en un `sudog`) |
| `lock` | un cerrojo interno que protege todo lo anterior |

La cabecera ocupa ≈ 96 bytes; si el tipo del elemento **no contiene punteros**, cabecera y buffer se reservan en **un único bloque** (el GC no necesita recorrer el buffer).

### Enviar y recibir

`c <- v` (`chansend`):
1. Toma el cerrojo. Si el channel está cerrado: pánico.
2. Si hay un **receptor esperando en `recvq`**: le entrega el valor **directamente** (se copia al stack del receptor, sin pasar por el buffer) y lo despierta (`goready`).
3. Si hay hueco en el buffer: lo copia ahí.
4. Si no, **aparca** la goroutine en `sendq` (`gopark`) hasta que un receptor la despierte.

`<-c` (`chanrecv`) es simétrico: primero busca un emisor en `sendq`, luego el buffer, y si no, se aparca en `recvq`.

`close(c)` despierta a **todos** los bloqueados: los receptores reciben el valor cero con `ok == false`; los emisores, un pánico.

### `select`

`select` con ≥ 2 casos llama a `runtime.selectgo`:

1. Baraja los casos con un orden **aleatorio** (por eso no hay prioridad ni sesgo).
2. **Ordena los channels por dirección** y **bloquea todos** (el orden global evita interbloqueos entre `select`s).
3. Busca un caso listo en el orden aleatorio; si lo hay, lo ejecuta, desbloquea y devuelve.
4. Si ninguno está listo y no hay `default`: encola un `sudog` en **cada** channel y aparca; al despertar, desencola de todos los demás.

Casos especiales que el compilador simplifica:

| Código | Se compila a |
|--|--|
| `select { case v := <-c: … }` (un caso) | un `<-c` normal |
| un caso + `default` | `selectnbrecv` / `selectnbsend` (operación **no bloqueante**, sin `selectgo`) |
| `select {}` | bloqueo permanente |

Un channel **`nil`** nunca está listo (útil para **desactivar** un caso dinámicamente); uno **cerrado** siempre lo está.

### Coste frente a un mutex

Un channel hace trabajo extra respecto a un `Mutex`: cerrojo interno, copia del dato, y —si hay que esperar— `gopark`/`goready` (un cambio de goroutine). Aun así es muy rápido; su valor está en **coordinar** (sincronizar + transferir propiedad), no en proteger un contador.

## Qué hace el programa

| Experimento | Qué mide |
|--|--|
| A | bytes por channel (sin buffer; `chan int` y `chan [64]byte` con buffer 1 000) |
| B | 8 receptores bloqueados en orden 0..7: ¿en qué orden se despiertan? |
| C | `select` con 3 casos siempre listos, 300 000 veces: ¿qué porcentaje elige cada uno? |
| D | coste de un `select` con 1, 2, 4 y 8 casos |
| E | ping-pong sin buffer, flujo con y sin buffer, y mutex+contador |
| F | `select` entre un channel `nil` y uno cerrado |

```bash
go run .
```

## Resultados de referencia (Apple M1, Go 1.26.5)

```
A) chan struct{}  (sin buffer)          111 bytes por channel
   chan int       (buffer 1000)        8191 bytes   (buffer teórico 8000 B)
   chan [64]byte  (buffer 1000)       65535 bytes   (buffer teórico 64000 B)
B) bloqueados en el orden 0..7 → despertados en el orden [0 1 2 3 4 5 6 7]
C) a: 33.30 %   b: 33.37 %   c: 33.33 %
D) 1 caso: 30.7 ns | 2: 60.9 | 4: 104.8 | 8: 276.8
E) sin buffer, ping-pong: 281 ns por viaje de ida y vuelta
   con buffer (1000), productor→consumidor: 73 ns/elemento
   sin buffer, productor→consumidor: 133 ns/elemento
   mutex + contador (2 goroutines): 43 ns/operación
F) cerrado elegido 1000 de 1000 (el nil nunca)
```

## Conclusiones

- **Cabecera de ≈ 111 bytes** (96 de `hchan` + redondeo del asignador); el buffer se suma en el mismo bloque (8 191 ≈ 8 000 + 111).
- **Cola FIFO de esperas:** los receptores se despiertan en el orden en que se bloquearon.
- **`select` es uniforme:** 33.3 % cada caso. No hay prioridad: si la necesitas, anida un `select` con `default`.
- **El coste de `select` crece con el nº de casos** (31 → 277 ns de 1 a 8): bloquea todos los channels, elige y desbloquea.
- **Un buffer reduce el coste del flujo a la mitad** (133 → 73 ns/elemento) porque evita aparcar y despertar en cada elemento.
- **Un mutex con contador (43 ns) es más barato que un channel (73-133 ns)** para proteger estado simple: usa channels para **transferir datos y coordinar**, mutex para **proteger estado**.

## Ejercicios propuestos

1. Verifica que el valor se entrega **directamente** al receptor: compara el coste de enviar a un receptor ya esperando frente a llenar el buffer.
2. Usa `reflect.Select` con 64 casos y mide su coste frente a un `select` escrito.
3. Demuestra que un `nil` channel desactiva un caso: implementa un bucle que funda dos channels y pone a `nil` el que se cierra.
4. Mide el coste de `close` con 1 000 receptores bloqueados.
