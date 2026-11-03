#!/usr/bin/env bash
# Mide el speedup al añadir contenedores worker (cada uno limitado a 1 CPU).
# Uso: ./scripts/escalar.sh [lista de réplicas]        p. ej.  ./scripts/escalar.sh "1 2 4 8"
# Variables opcionales: TAREAS (800), ITER (10000000), CONC (64), SERIAL (0)
set -euo pipefail
cd "$(dirname "$0")/.."
REPLICAS=${1:-"1 2 4 8"}
export TAREAS=${TAREAS:-800} ITER=${ITER:-10000000} CONC=${CONC:-64} SERIAL=${SERIAL:-0}

docker compose build -q worker
printf "%-8s %-10s %-12s %-9s %-10s %s\n" workers total tareas/s speedup eficiencia "workers que respondieron"
base=""
for n in $REPLICAS; do
  docker compose up -d --scale worker="$n" --wait worker >/dev/null 2>&1
  salida=$(docker compose run --rm -T --no-deps cliente 2>&1)
  ms=$(echo "$salida" | sed -n 's/^RESULTADO total_ms=\([0-9]*\) .*/\1/p')
  resp=$(echo "$salida" | sed -n 's/^RESULTADO .*workers=\([0-9]*\)/\1/p')
  [ -z "$ms" ] && { echo "$salida"; exit 1; }
  [ -z "$base" ] && base=$ms
  printf "%-8s %-10s %-12s %-9s %-10s %s\n" "$n" "$(awk -v m="$ms" 'BEGIN{printf "%.2f s", m/1000}')" \
    "$(awk -v m="$ms" -v t="$TAREAS" 'BEGIN{printf "%.0f", t*1000/m}')" \
    "$(awk -v b="$base" -v m="$ms" 'BEGIN{printf "%.2fx", b/m}')" \
    "$(awk -v b="$base" -v m="$ms" -v n="$n" 'BEGIN{printf "%.0f%%", 100*b/m/n}')" "$resp"
done
docker compose down >/dev/null 2>&1
