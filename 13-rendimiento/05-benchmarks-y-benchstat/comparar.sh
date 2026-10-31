#!/bin/sh
# Compara "antes" (RWMutex) y "después" (fragmentado) con significancia estadística.
#   ./comparar.sh            # 10 repeticiones de cada una (≈ 2 min)
#   COUNT=6 ./comparar.sh
set -e
cd "$(dirname "$0")"
COUNT=${COUNT:-10}
OUT=${OUT:-$(mktemp -d)}

if command -v benchstat >/dev/null 2>&1; then
  BENCHSTAT=benchstat
else
  BENCHSTAT="go run golang.org/x/perf/cmd/benchstat@latest"
fi

echo "resultados en $OUT"
IMPL=rw    go test -bench 'Almacen' -run xxx -count "$COUNT" -cpu 8 > "$OUT/antes.txt"
IMPL=shard go test -bench 'Almacen' -run xxx -count "$COUNT" -cpu 8 > "$OUT/despues.txt"
IMPL=rw    go test -bench 'Almacen' -run xxx -count "$COUNT" -cpu 8 > "$OUT/antes2.txt"

cd "$OUT" # (así las cabeceras de columna son los nombres cortos de los archivos)
echo
echo "== antes (rw) frente a después (shard) =="
$BENCHSTAT antes.txt despues.txt
echo
echo "== antes frente a antes (MISMO código dos veces: no debe haber diferencia relevante) =="
$BENCHSTAT antes.txt antes2.txt
