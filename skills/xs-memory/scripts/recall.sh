#!/usr/bin/env bash
# CLI shim for recall — for non-MCP setups.
# Usage: recall.sh <query> [--store path] [--collection name]
set -euo pipefail

STORE="${XSMEM_STORE:-~/.xsmem/default.xsmem}"
COLLECTION="default"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --store) STORE="$2"; shift 2 ;;
    --collection) COLLECTION="$2"; shift 2 ;;
    *) QUERY="$1"; shift ;;
  esac
done

if [ -z "${QUERY:-}" ]; then
  echo "Usage: recall.sh <query> [--store path] [--collection name]" >&2
  exit 1
fi

exec xsmem search --store "$STORE" --collection "$COLLECTION" --mode hybrid "$QUERY"
