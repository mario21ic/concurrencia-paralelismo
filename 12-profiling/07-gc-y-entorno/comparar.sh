#!/bin/sh
# Compara configuraciones de GC con la misma carga. Cada configuración se repite
# 3 veces; se imprime una línea por ejecución.
set -e
cd "$(dirname "$0")"
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
go build -o "$TMP/greentea" .
GOEXPERIMENT=nogreenteagc go build -o "$TMP/clasico" .

echo "== Efecto de GOGC (recolector por defecto de Go $(go version | awk '{print $3}'): Green Tea) =="
for g in 25 100 400; do
  for i in 1 2 3; do "$TMP/greentea" -gogc $g; done
done
echo
echo "== GOGC desactivado + límite de memoria (GOMEMLIMIT) =="
for l in 400 120; do
  for i in 1 2; do "$TMP/greentea" -gogc -1 -memlimit $l; done
done
echo
echo "== Green Tea (por defecto) frente a GC clásico (GOEXPERIMENT=nogreenteagc), GOGC=100 =="
for i in 1 2 3; do printf "green tea : "; "$TMP/greentea" -gogc 100; done
for i in 1 2 3; do printf "clásico   : "; "$TMP/clasico"  -gogc 100; done
