# Ejercicio 6: el servidor más rápido, con cancelación real

## Enunciado

Reescribir el ejercicio del módulo 4 (`04-select-timers/07-servidor-mas-rapido`) para que, al obtener el primer ganador, los demás servidores **dejen de trabajar**, no solo de ser esperados.

## Teoría y diseño

```
ctx ──► WithCancel ──► servidor 1 ┐
                    ─► servidor 2 ├─► [ch buffer=3] ──► ganador
                    ─► servidor 3 ┘
              ▲
              └── cancel() al recibir el primero
```

- Cada consulta usa `select { case <-time.After(latencia): ...; case <-ctx.Done(): ... }`; al cancelar, abandona su espera al instante.
- `defer cancel()` en `primero` garantiza que ninguna goroutine sobreviva a la función, ni siquiera si hay un error.
- `wg.Wait()` tras cancelar: la función no retorna hasta que todas terminaron (**concurrencia estructurada**: nada se escapa del ámbito que lo lanzó).
- El buffer del channel sigue siendo útil: un perdedor que ya tenía respuesta lista no se bloquea al enviar.
- Se ejecuta con `cancelar=false` (contexto que nunca termina) para comparar.

## Cómo ejecutarlo

```bash
go run .
go run -race .
```

## Resultado de referencia

```
cancelar=false: ganó "datos de us-este"; todas las goroutines terminaron a los 500ms
  eu-oeste  trabajó 300ms  completó
  us-este   trabajó 100ms  completó
  asia      trabajó 500ms  completó

cancelar=true: ganó "datos de us-este"; todas las goroutines terminaron a los 100ms
  eu-oeste  trabajó 100ms  CANCELADO
  us-este   trabajó 100ms  completó
  asia      trabajó 100ms  CANCELADO
```

## Conclusiones

- Sin cancelación hay que esperar al más lento (500 ms) para limpiar; y en la vida real los perdedores seguirían consumiendo CPU, conexiones y dinero.
- Con cancelación todo termina cuando termina el ganador (100 ms).
- Esto resuelve la limitación declarada en el módulo 4.

## Ejercicios propuestos

1. Añade un `WithTimeout` global y verifica que cancela a todos si no gana nadie a tiempo.
2. Haz que los servidores sean peticiones HTTP reales con `httptest` y comprueba que los perdedores ven `r.Context().Done()`.
3. Con `WithCancelCause`, registra en la traza *por qué* cada servidor fue cancelado ("perdió ante us-este").
