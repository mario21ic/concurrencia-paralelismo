#!/usr/bin/env bash
# Mide el speedup al añadir contenedores worker (cada uno limitado a 1 CPU).
# Uso: ./scripts/escalado.sh [N_TRABAJOS] [RONDAS] [lista de réplicas]
#   ./scripts/escalado.sh 300 600000 "1 2 4 8"
set -euo pipefail
cd "$(dirname "$0")/.."
N=${1:-300}
RONDAS=${2:-600000}
REPLICAS=${3:-"1 2 4 8"}

docker compose up -d --build coordinador >/dev/null
printf "%-8s %-12s %-12s %-10s %-10s\n" workers "total" "jobs/s" "speedup" "eficiencia"
base=""
for r in $REPLICAS; do
  docker compose up -d --scale worker="$r" --no-recreate worker >/dev/null 2>&1
  docker compose up -d --scale worker="$r" worker >/dev/null 2>&1
  sleep 4   # los workers nuevos tardan un instante en empezar a arrendar
  salida=$(docker compose run --rm -T loadgen /app/loadgen -n "$N" -tipo hash \
           -payload "{\"rondas\":$RONDAS}" -concurrencia 64 2>/dev/null)
  tps=$(echo "$salida" | sed -n 's/.*throughput: \([0-9.]*\) jobs\/s.*/\1/p')
  total=$(echo "$salida" | sed -n 's/.*total: \([0-9.]*[a-zµ]*\) |.*/\1/p')
  [ -z "$base" ] && base=$tps
  printf "%-8s %-12s %-12s %-10s %-10s\n" "$r" "$total" "$tps" \
    "$(echo "$tps $base" | awk '{printf "%.2fx", $1/$2}')" \
    "$(echo "$tps $base $r" | awk '{printf "%.0f%%", 100*$1/$2/$3}')"
done
