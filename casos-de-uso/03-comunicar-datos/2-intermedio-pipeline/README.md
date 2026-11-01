# Caso 3.2 — Pipeline de log con cancelación  ·  `intermedio`

> **Problema:** Procesar un flujo de líneas de log: leer → parsear → agregar. Cada etapa es una goroutine que lee de la anterior y escribe en la siguiente; se solapan, y todo se detiene limpiamente al cancelar.

## Qué features usa y por qué

| Feature de Go | Papel en este caso |
|--|--|
| Etapas con `<-chan` | cada función devuelve su canal de salida y cierra al terminar |
| `select` con `ctx.Done()` en cada envío | si el consumidor se va, ninguna etapa queda bloqueada |
| `atomic.Int64` | contador de líneas inválidas compartido entre etapas |
| `goleak` | el test confirma que no quedan goroutines |

## ¿Cuándo usarlo? ¿Cuándo no?

- ✅ Datos que fluyen y se transforman por etapas **independientes**.
- ✅ Cada etapa puede ser lenta o rápida: los channels las desacoplan.
- ❌ Etapas muy baratas: el coste de los channels (~50-100 ns por elemento) supera al trabajo → procesa en lotes (módulo 13).
- ❌ Pocos datos: una función secuencial es más clara.

## Cómo ejecutarlo

```bash
go run .
go test -race -v .
```

## Qué observar (salida real, Apple M1, Go 1.26.5)

```
resumen: map[api:{2 200} cache:{1 5} db:{2 100}] | líneas inválidas: 2
con ctx cancelado: map[api:{1 120}] (resultado parcial; terminó limpiamente)
```

## Errores típicos

- Enviar sin `select` con `ctx.Done()`: al cancelar o abandonar, la etapa queda bloqueada para siempre (**fuga**).
- No cerrar el channel de salida (`defer close(out)`): la etapa siguiente no termina.
- Una etapa lenta sin buffer frena a todas (es contrapresión natural, a veces lo que quieres).
- Compartir el mapa de resultados entre etapas: que lo construya SOLO la última.

## Profundiza en el curso

- [pipelines a fondo](../../../08-patrones/03-pipeline)
- [errores dentro de un pipeline](../../../10-errores/05-errores-en-pipelines)
- [cancelación cooperativa](../../../05-context/03-cancelacion-cooperativa)
- [detectar fugas de goroutines](../../../11-testing/05-fugas)
- [¿iterador o channel?](../../../09-iteradores/03-iteradores-vs-canales)

## Prueba tú

1. Añade una 4.ª etapa de filtrado.
2. Haz que el parseo use 4 goroutines (pista: caso 3.3).
3. Mide cuántas líneas/s procesa y cuánto sube agrupando de 100 en 100.
