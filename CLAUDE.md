# CLAUDE.md

This file is the operating guide that Claude Code reads at the start of every session. **Keep it short.**
Detailed conventions and design live in the two files below; this file covers only day-to-day operations and the most critical guardrails.

@PROJECT.md
@docs/xs-memory-design.md

---

## What This Repository Is

`xs-memory`: An embedded memory engine for local AI agents ("SQLite for agents").
Pure Go, single binary. Full-text / vector / hybrid search + small-scale knowledge graph. Provides CLI and MCP.

Only **MVP (v0.1)** may be implemented right now (PROJECT.md §7). Anything beyond that should remain a proposal.

---

## Commands to Use First

```bash
make build         # Pure Go build with CGO_ENABLED=0 (default)
make test          # go test ./... -race
make lint          # golangci-lint
make fmt           # gofumpt + goimports
make bench         # Search latency / cache hit rate
make run -- <args> # Local execution
make mcp-init      # Build binary + initialize .xsmem/dev.xsmem store
```

- **Before every commit/PR**: Make `make fmt && make lint && make test` green.
- The default build is **`CGO_ENABLED=0`**. Features requiring CGO are explicitly isolated with `-tags <name>`.

---

## Rules That Must Never Be Broken (Summary)

The full list is PROJECT.md §3 (N1–N9). These five have top priority:

1. **Pure Go, single binary.** CGO dependencies (tree-sitter, etc.) are isolated by build tag and excluded from the default.
2. **Explicit memory budget control.** Block cache LRU guarantees the upper bound. Never implement "load everything into RAM."
3. **No LLM on the critical path.** Organization is async and optional. Search must work even when unconfigured.
4. **Deletion defaults to soft** (tombstone). Physical deletion only with an explicit flag.
5. **The public API is `xsmem` only.** UIs never touch `internal/` directly.

When in doubt, read the relevant design doc section before writing code. Do not introduce major decisions not in the design doc — present them as proposed additions to the Decision Log.

---

## Work Loop

**Explore → Plan → Implement → Verify**, in that order. Never rewrite broad swaths of code without preparation.

1. **Explore**: Read related files and their tests before touching anything. Match existing conventions and naming.
2. **Plan**: Briefly state what you'll change and in what order. Indicate which design doc section it corresponds to.
3. **Implement**: One task, one concern. Behavior-changing modifications come **with tests**.
4. **Verify**: Run `make test` (with `-race`) and report results.

For destructive or wide-ranging operations (bulk file changes, deletions, `go mod` overhauls, adding dependencies), **present a diff summary and get confirmation before executing**.

---

## Code Conventions (Key Points Only — Details in PROJECT.md §8)

- Wrap errors with `%w`. Libraries never panic.
- External I/O, LLM, and search take `context.Context` as the first argument.
- Logging uses `log/slog`. **Never log API keys, full content bodies, or PII.**
- "Accept interfaces, return structs." `storage.Engine` / `provider.Embedder` / `provider.LLM` / `analyzer.Analyzer` are interface-abstracted.
- Comment the why, and cite design decisions by section number (e.g., `// see design §6.3`).
- Formatting: gofumpt + goimports.

---

## Pitfalls Specific to This Codebase

- **Always include Japanese in tests.** Tokenization (Kagome) and search break easily with CJK. Never validate with English only.
- **Embedding model/dimensions are stamped on the collection and immutable.** Do not change dimensions after the fact in tests or implementation.
- **Do not mix external APIs into tests.** Use `provider/mock` for embeddings/LLM. Real APIs only under `-tags integration`.
- **LRU eviction and semantic forgetting (Organizer) are different things.** Conflating them leads to data-loss bugs.
- **Measure before optimizing vector strategy.** MVP uses flat + int8 quantization. Do not add HNSW on your own (design doc D4).
- When touching WAL / segments / compaction, always update crash recovery and LRU tests.

---

## Local xs-memory MCP

This project uses its own `xsmem` binary as an MCP server for persistent agent memory.
Config is in `.mcp.json`; the store lives at `.xsmem/dev.xsmem`.

- **First-time setup**: `make mcp-init` (builds binary + creates store if missing).
- **After code changes to xsmem**: `make build` to update `bin/xsmem` so the MCP server picks up the latest.
- **Skill**: `/xs-memory` — recall, remember, organize, forget. See `skills/xs-memory/SKILL.md`.
- **Cleanup**: `xsmem clean --store .xsmem/dev.xsmem` to wipe the store.

---

## When to Stop and Ask

- A design decision not covered by the design doc is needed.
- A non-negotiable (PROJECT.md §3) is being touched or at risk of being violated.
- Destructive or wide-ranging changes.
- An API specification is unclear (never fill in by guessing).
- You want to add a new dependency (present rationale, alternatives, license, and CGO status).
