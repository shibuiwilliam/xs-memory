#!/usr/bin/env bash
# Smoke test for xs-memory MVP.
# Usage: scripts/smoke.sh [path-to-xsmem-binary]
set -euo pipefail

XSMEM="${1:-bin/xsmem}"
STORE=$(mktemp -d)/smoke.xsmem

echo "=== xs-memory smoke test ==="
echo "Binary: $XSMEM"
echo "Store:  $STORE"
echo

# 1. Init
echo "--- init ---"
$XSMEM init "$STORE" --analyzer ja --collection default
echo

# 2. Add English
echo "--- add (English) ---"
echo "Go is a statically typed, compiled programming language designed at Google." | $XSMEM add --store "$STORE" --type semantic -
echo

# 3. Add Japanese
echo "--- add (Japanese) ---"
echo "東京タワーは東京都港区にある電波塔です。高さは333メートルあります。" | $XSMEM add --store "$STORE" --type semantic -
echo

echo "--- add (Japanese 2) ---"
echo "京都の金閣寺は美しい寺院で、多くの観光客が訪れます。" | $XSMEM add --store "$STORE" --type episodic -
echo

echo "--- add (code) ---"
echo 'func main() { fmt.Println("Hello, World!") }' | $XSMEM add --store "$STORE" --type procedural --source "file://main.go" -
echo

# 4. List
echo "--- ls ---"
$XSMEM ls --store "$STORE"
echo

# 5. Search FTS
echo "--- search (fts) ---"
$XSMEM search --store "$STORE" --mode fts "programming"
echo

# 6. Search FTS Japanese
echo "--- search (fts, Japanese) ---"
$XSMEM search --store "$STORE" --mode fts "東京"
echo

# 7. Search Hybrid
echo "--- search (hybrid) ---"
$XSMEM search --store "$STORE" --mode hybrid "電波塔"
echo

# 8. Stats
echo "--- stats ---"
$XSMEM stats --store "$STORE"
echo

# 9. Get first memory
echo "--- get (first) ---"
FIRST_ID=$($XSMEM ls --store "$STORE" --json | python3 -c "import sys,json; print(json.load(sys.stdin)[0]['id'])" 2>/dev/null || echo "")
if [ -n "$FIRST_ID" ]; then
    $XSMEM get --store "$STORE" "$FIRST_ID"
    echo

    # 10. Update
    echo "--- update ---"
    $XSMEM update --store "$STORE" "$FIRST_ID" --importance 0.9
    echo

    # 11. Remove (soft)
    echo "--- rm (soft) ---"
    $XSMEM rm --store "$STORE" "$FIRST_ID"
    echo
fi

# 12. Export/Import roundtrip
ARCHIVE=$(dirname "$STORE")/export.tar.gz
IMPORT_STORE=$(dirname "$STORE")/imported.xsmem

echo "--- export ---"
$XSMEM export --store "$STORE" "$ARCHIVE"
echo

echo "--- import ---"
$XSMEM import --store "$IMPORT_STORE" "$ARCHIVE"
echo

echo "--- imported store stats ---"
$XSMEM stats --store "$IMPORT_STORE"
echo

# 13. Final stats
echo "--- final stats ---"
$XSMEM stats --store "$STORE"
echo

# Cleanup
rm -rf "$(dirname "$STORE")"

echo "=== smoke test PASSED ==="
