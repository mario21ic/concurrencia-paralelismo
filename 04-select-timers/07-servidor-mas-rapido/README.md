# Ejercicio 7: el servidor más rápido

## Enunciado

Consultar 3 servidores simulados (réplicas) en paralelo y devolver la **primera respuesta correcta**. Debe:

- ignorar respuestas con error mientras quede alguna esperanza,
- aplicar un timeout global,
- no dejar goroutines colgadas.

Es un patrón real: *hedged requests*, consulta a réplicas, DNS, caches multi-región.

## Teoría y diseño

```
        ┌─► servidor "eu-oeste" ─┐
main ───┼─► servidor "us-este"  ─┼─► [res, buffer=3] ──► primero que no sea error
        └─► servidor "asia"     ─┘          │
                                       timer global
```

1. Una goroutine por servidor; todas escriben en **un solo channel `res`**.
2. **`res` tiene capacidad `len(servidores)`**: los servidores que pierden pueden enviar su respuesta y terminar, aunque nadie las lea. Con un channel sin buffer quedarían bloqueados para siempre (fuga).
3. Un `Timer` único fuera del bucle impone el **plazo global**; `defer plazo.Stop()`.
4. Los errores se acumulan con `errors.Join`; si todos fallan o expira el plazo, se devuelve un error con el contexto.
5. **Limitación consciente:** los servidores perdedores siguen trabajando hasta terminar; el timeout solo deja de esperarlos. Cancelarlos de verdad requiere `context` (módulo 5).

## Cómo ejecutarlo

```bash
go run .
go run -race .
```

## Resultado de referencia

```
#1 ganó us-este   en 69ms -> datos de us-este
#2 ganó us-este   en 52ms -> datos de us-este
#3 ganó us-este   en 46ms -> datos de us-este
#4 ganó asia      en 169ms -> datos de asia      <- us-este y eu-oeste fallaron
#5 ganó us-este   en 45ms -> datos de us-este
timeout: timeout de 100ms

goroutines fugadas con buffer: 0
goroutines fugadas sin buffer: 20 (2 por llamada)
```

(Los ganadores y tiempos varían: hay latencia y fallos aleatorios.)

## Conclusiones

- El tiempo de respuesta es el del **más rápido que acierte**, no el de la suma ni el del más lento.
- En la ejecución #4 los dos más rápidos fallaron y ganó `asia`: por eso se espera una respuesta **correcta**, no la primera.
- El buffer del channel marcó la diferencia entre 0 y 20 goroutines fugadas tras 10 llamadas.

## Ejercicios propuestos

1. Pasa `pFallo` a 1.0 en todos y verifica el mensaje agregado.
2. Añade un cuarto servidor y comprueba que la capacidad del buffer sigue siendo correcta sin tocar nada más.
3. Haz que, en cuanto haya ganador, se cierre un canal `cancelar` que los servidores simulados observen para dejar de dormir (adelanto de `context`).
4. Escribe un test con `-race` que fije la semilla de aleatoriedad y compruebe que gana el servidor esperado.
