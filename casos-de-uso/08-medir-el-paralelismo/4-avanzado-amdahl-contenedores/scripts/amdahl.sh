#!/usr/bin/env bash
# Ley de Amdahl con contenedores: la fracción serial se ejecuta en el cliente (un solo proceso) y el
# resto se reparte entre N workers. Reutiliza el compose del caso 7.3.
# Uso: ./scripts/amdahl.sh ["fracciones seriales"] ["réplicas"]   p. ej.  ./scripts/amdahl.sh "0 0.10 0.25 0.50" "1 2 4 8"
set -euo pipefail
cd "$(dirname "$0")/../../../07-io-vs-cpu/3-avanzado-compose-escalado"
SERIALES=${1:-"0 0.10 0.25 0.50"}
REPLICAS=${2:-"1 2 4 8"}
export TAREAS=${TAREAS:-400} ITER=${ITER:-20000000} CONC=${CONC:-64}

docker compose build -q worker
echo "Cada celda: aceleración medida (teórica de Amdahl) frente a 1 worker con la misma fracción serial."
printf "%-9s %-10s" "serial" "T(1)"
for n in $REPLICAS; do [ "$n" = 1 ] || printf " %-16s" "N=$n"; done; echo
for s in $SERIALES; do
  export SERIAL=$s
  base=""; fila=""
  for n in $REPLICAS; do
    docker compose up -d --scale worker="$n" --wait worker >/dev/null 2>&1
    ms=$(docker compose run --rm -T --no-deps cliente 2>&1 | sed -n 's/^RESULTADO total_ms=\([0-9]*\) .*/\1/p')
    [ -z "$ms" ] && { echo "fallo midiendo s=$s n=$n"; exit 1; }
    if [ -z "$base" ]; then
      base=$ms
      printf "%-9s %-10s" "$(awk -v s="$s" 'BEGIN{printf "%.0f %%", 100*s}')" "$(awk -v m="$ms" 'BEGIN{printf "%.2f s", m/1000}')"
    else
      printf " %-16s" "$(awk -v b="$base" -v m="$ms" -v s="$s" -v n="$n" 'BEGIN{printf "%.2fx (%.2fx)", b/m, 1/(s+(1-s)/n)}')"
    fi
  done
  echo
done
docker compose down >/dev/null 2>&1
