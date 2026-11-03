# Caso 8.4 — La ley de Amdahl con contenedores  ·  `avanzado`

> **Problema:** En [8.2](../2-intermedio-amdahl) medimos Amdahl con goroutines en un proceso. ¿Se cumple igual cuando el paralelismo viene de **añadir contenedores**, y dónde vive la parte serial?

Este caso **no tiene código propio**: reutiliza el `compose.yaml` y el binario de [7.3](../../07-io-vs-cpu/3-avanzado-compose-escalado). Solo añade un script y el análisis.

## Cómo está montado

El cliente recibe `SERIAL=s`. Antes de repartir, ejecuta **él mismo y en una sola goroutine** la fracción `s` del trabajo total; el resto (1−s) se reparte entre N workers de 1 CPU. El trabajo total es el mismo para cualquier `s`. Es exactamente el modelo de Amdahl:

```
T(N) = s·T₁  +  (1−s)·T₁ / N              speedup(N) = 1 / ( s + (1−s)/N )
```

En un sistema real esa parte serial sería lo que hace el coordinador (validar, particionar, fusionar resultados, escribir la salida): cuanto más pesa, antes se aplana la curva por muchos workers que añadas.

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ Para **decidir si añadir réplicas merece la pena** antes de pagarlas: mide (o estima) qué fracción es serial.
- ✅ Para ver que el techo `1/s` es independiente de si usas goroutines, procesos o máquinas.
- ❌ Aquí la parte serial es CPU del cliente; en sistemas reales también hay **latencia de red** y **serialización** en el camino que Amdahl no modela.

## Cómo ejecutarlo

```bash
cd casos-de-uso/08-medir-el-paralelismo/4-avanzado-amdahl-contenedores
./scripts/amdahl.sh                                   # ≈ 2-3 min
./scripts/amdahl.sh "0 0.25" "1 2 4"                  # menos casos
```

## Qué observar (salida real, Apple M1, Docker Desktop con 8 CPU)

400 tareas de 20 M iteraciones; cada celda es la aceleración medida frente a 1 worker **con la misma fracción serial**, y entre paréntesis la teórica de Amdahl:

```
serial    T(1)       N=2              N=4              N=8             
0 %       18.63 s    2.11x (2.00x)    3.87x (4.00x)    5.96x (8.00x)   
10 %      18.62 s    1.97x (1.82x)    3.39x (3.08x)    4.10x (4.71x)   
25 %      17.46 s    1.69x (1.60x)    2.35x (2.29x)    2.79x (2.91x)   
50 %      16.89 s    1.39x (1.33x)    1.56x (1.60x)    1.83x (1.78x)   
```

- **Concuerda con la teoría** mucho mejor que en 8.2: con `s = 25 %`, 2.35× frente a 2.29× (N=4) y 2.79× frente a 2.91× (N=8); con `s = 50 %`, 1.83× frente a 1.78×. Con procesos separados no hay contención de caché ni de memoria entre workers.
- **La fila `s = 0 %` es el techo real del sistema**: 5.96× con 8 workers, no 8×. Las desviaciones en `s = 10 %` (4.10× frente a 4.71×) vienen de ahí.
- **Cada fila mide con su propio `N=1`**, que incluye el rozamiento de 64 peticiones concurrentes en un único contenedor de 1 CPU (ver [7.3](../../07-io-vs-cpu/3-avanzado-compose-escalado)); por eso `N=2` con `s = 0 %` sale ligeramente por encima de 2× (2.11×).
- Con `s = 50 %` pasar de 4 a 8 contenedores (**1.56× → 1.83×**) casi no compensa. Si tu sistema se parece a esa fila, **añadir workers es tirar el dinero: hay que reducir la parte serial**.

## Errores típicos

- Escalar workers sin medir qué fracción es serial en el coordinador o en el cliente.
- Dejar al coordinador hacer trabajo pesado «porque es poco»: con `s = 10 %` el techo ya es 10×, y 8 workers dan 4.1×.
- Comparar contra `N=1` de otra configuración: cada fila debe usar su propia base.
- Olvidar que el techo real de la máquina (aquí 5.96×) está por debajo de `N`.

## Profundiza en el curso

- [8.2 — Amdahl con goroutines](../2-intermedio-amdahl)
- [7.3 — la infraestructura que usa este caso](../../07-io-vs-cpu/3-avanzado-compose-escalado)
- [la ley de Amdahl, con ejercicios](../../../01-fundamentos/05-amdahl)
- [proyecto final](../../../proyecto-final): el coordinador es la parte «serial» del sistema

## Prueba tú

1. Añade `s = 5 %` y `s = 75 %` y comprueba la forma de la curva.
2. Pon `CONC=8` en vez de 64: ¿cambia la base `N=1`? (exporta la variable antes de `./scripts/amdahl.sh`).
3. Haz que la parte serial sea una espera en lugar de CPU (un `time.Sleep` en el cliente): ¿se comporta igual? ¿por qué un cliente con espera no consume CPU?
