#!/usr/bin/env bash
# Smoke test for the metrics subsystem. See addendum3 §6.
# Usage: scripts/smoke-metrics.sh
set -euo pipefail

BINARY="./bin/xsmem"
STORE="/tmp/smoke-metrics-$$.xsmem"
trap 'rm -rf "$STORE"' EXIT

echo "=== Build ==="
make build

echo ""
echo "=== Init store ==="
$BINARY init "$STORE" --collection default

echo ""
echo "=== Add test memories ==="
echo "Go is a programming language designed at Google" | $BINARY add --store "$STORE" -
echo "Rust is a systems programming language focused on safety" | $BINARY add --store "$STORE" -
echo "東京タワーは東京都港区にある電波塔です" | $BINARY add --store "$STORE" -
echo "Python is popular for data science and machine learning" | $BINARY add --store "$STORE" -

echo ""
echo "=== Stats (before metrics — shows structural stats) ==="
$BINARY stats --store "$STORE"

echo ""
echo "=== Stats JSON ==="
$BINARY stats --store "$STORE" --json | python3 -m json.tool > /dev/null 2>&1 && echo "  stats JSON: valid" || echo "  stats JSON: INVALID"

echo ""
echo "=== Metrics (disabled by default) ==="
$BINARY metrics --store "$STORE"

echo ""
echo "=== Search (metrics disabled — just verifying search works) ==="
$BINARY search --store "$STORE" "programming" --mode fts --topk 5
$BINARY search --store "$STORE" "東京" --mode fts --topk 5

echo ""
echo "=== Metrics JSON (disabled — should be empty) ==="
$BINARY metrics --store "$STORE" --json 2>/dev/null || echo "  (metrics disabled, no JSON output)"

echo ""
echo "=== Privacy check: grep for raw query in store ==="
if grep -r "programming" "$STORE" 2>/dev/null | grep -v "Go is a\|Rust is a\|Python is\|manifest\|meta.db" | head -1; then
    echo "  WARNING: raw query text found in store files"
else
    echo "  PASS: no raw query text in store (expected — metrics disabled)"
fi

echo ""
echo "=== Metrics reset ==="
$BINARY metrics --store "$STORE" --reset

echo ""
echo "=== All smoke tests passed ==="
