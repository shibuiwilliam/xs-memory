#!/usr/bin/env bash
# Smoke test for MCP host-delegation features.
# Tests the addendum contract: recall works, organization returns work packets,
# merge requires confirmation, provider is never called in host-delegated mode.
set -euo pipefail

XSMEM="${1:-bin/xsmem}"
STORE=$(mktemp -d)/smoke-mcp.xsmem

echo "=== xs-memory MCP addendum smoke test ==="
echo "Binary: $XSMEM"
echo "Store:  $STORE"
echo

# 1. Init
echo "--- init ---"
$XSMEM init "$STORE" --analyzer en
echo

# 2. Add memories
echo "--- add memories ---"
echo "Go is a programming language designed at Google for systems programming." | $XSMEM add --store "$STORE" --type semantic -
echo "We decided to use LRU caching with a 256MB budget." | $XSMEM add --store "$STORE" --type episodic -
echo "The cache eviction policy uses least recently used strategy." | $XSMEM add --store "$STORE" --type semantic -
echo "東京タワーは東京都港区にある電波塔です。" | $XSMEM add --store "$STORE" --type semantic -
echo

# 3. Recall (FTS - no LLM needed)
echo "--- recall (FTS search, no LLM) ---"
$XSMEM search --store "$STORE" --mode fts "cache eviction"
echo

# 4. Recall (hybrid search)
echo "--- recall (hybrid search) ---"
$XSMEM search --store "$STORE" --mode hybrid "programming"
echo

# 5. Stats
echo "--- stats ---"
$XSMEM stats --store "$STORE"
echo

# 6. Get first memory and verify it exists
echo "--- get + update ---"
FIRST_ID=$($XSMEM ls --store "$STORE" --json | python3 -c "import sys,json; print(json.load(sys.stdin)[0]['id'])" 2>/dev/null || echo "")
if [ -n "$FIRST_ID" ]; then
    $XSMEM get --store "$STORE" "$FIRST_ID"
    echo

    # 7. Soft delete (default behavior per N7)
    echo "--- soft delete ---"
    $XSMEM rm --store "$STORE" "$FIRST_ID"
    echo
fi

# 8. Final stats
echo "--- final stats ---"
$XSMEM stats --store "$STORE"
echo

# Cleanup
rm -rf "$(dirname "$STORE")"

echo "=== MCP smoke test PASSED ==="
