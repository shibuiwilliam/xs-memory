# PROJECT.md — small-memory

> This file is the project's "constitution," read first by both AI coding agents (Claude Code) and human developers.
> For detailed design, [`docs/small-memory-design.md`](docs/small-memory-design.md) is the source of truth. If the two conflict, the design document takes precedence — update `PROJECT.md` accordingly.

---

## 0. Top-Priority Instructions for the Agent (Read First)

1. **Always read the relevant design document section before writing code.** Do not introduce major design decisions not covered by the design doc. If needed, first propose an addition to the "Decision Log."
2. **Do not violate the non-negotiables in §3.** Especially "pure Go, single binary," "CGO isolated by build tag," and "explicit memory budget control."
3. **Ship small.** Do not implement beyond the MVP scope (§7). Scope expansion follows the sequence: propose → agree → implement.
4. **Ship with tests.** Any behavior-changing modification must include tests (§6).
5. **When uncertain, stop and ask.** For destructive operations (deletions, schema changes, large refactors), summarize intent and diff before executing, and get confirmation.

---

## 1. Project Overview

`small-memory` is an **embedded memory engine** for locally-running AI agents.
It aims to be "SQLite for AI agents." No external server required. Single binary. Multi-platform.

- Memory **registration, organization, search, update, and deletion (forgetting)**.
- Search: **Full-text search (BM25) / vector / hybrid (RRF) / small-scale knowledge graph**.
- Interfaces: **CLI** and **MCP**. Web UI is a **plugin** (separated by build tag).
- Uses **LLM** (OpenAI / Claude / Gemini / Ollama) for data organization — asynchronously and optionally.

For details, diagrams, data model, and API sketch, see the design document.

---

## 2. Technology Stack and Assumptions

- **Language**: Go 1.22+
- **OS/Arch**: linux/macos/windows × amd64/arm64 (cross-compilation required)
- **Key libraries (candidates)**:
  - CLI: `spf13/cobra`
  - Meta/Graph KV: `go.etcd.io/bbolt`
  - Japanese morphology: `ikawaha/kagome` (pure Go)
  - FST dictionary: `blevesearch/vellum`
  - ID: `oklog/ulid`
  - MCP: `mark3labs/mcp-go` (or official Go SDK)
  - Logging: standard `log/slog`
- **Adding new dependencies requires review.** Include rationale, alternatives considered, license, and CGO status in the PR description.

---

## 3. Non-negotiables

Corresponds to design document Decision Log (D1–D9). **Changes that violate these are rejected by default.**

| # | Rule | Notes |
|---|---|---|
| N1 | **Pure Go by default, runs as a single binary** | CGO dependencies (e.g., tree-sitter) must always be isolated via **build tag** and excluded from the default build. |
| N2 | **Explicit memory budget control** | Block cache LRU guarantees an upper bound. Never implement "load everything into RAM." mmap is an `--mmap` advanced option. |
| N3 | **Core is a library (`smem`); UIs are thin adapters** | CLI/MCP/Web only call `smem`. Never touch the engine layer directly. |
| N4 | **No LLM on the critical path** | LLM organization is an async job, optional. Search works even when unconfigured (degraded mode). |
| N5 | **Don't break: WAL + immutable segments + CRC** | Writes are serialized through the WAL. Recovery via replay. |
| N6 | **Embedding model/dimensions are stamped on the collection and immutable** | Changes only via a rebuild command. |
| N7 | **Deletion defaults to soft (tombstone/archive)** | Physical deletion only with an explicit flag. Prevents destruction by LLM auto-organization. |
| N8 | **Don't retrofit CJK** | Analyzer is fixed per collection. Japanese (Kagome) is one of the defaults. |
| N9 | **No `internal/` boundary crossing** | Public API is `smem` only. Protects stability. |

---

## 4. Repository Layout

```
small-memory/
├── PROJECT.md                # this file
├── README.md                 # user-facing
├── Makefile                  # development tasks (§5)
├── go.mod
├── cmd/smem/                 # CLI entry point
├── smem/                     # Public Core API (Open/Store/Search...) ← only public package
├── internal/
│   ├── storage/              # WAL, segment, blockcache (LRU), bbolt wrapper
│   ├── index/
│   │   ├── fts/              # inverted index, BM25, analyzer integration
│   │   ├── vector/           # flat/quantize/(later hnsw)
│   │   └── graph/            # triples (SPO/POS/OSP), traversal
│   ├── ingest/               # parse, chunk, pipeline
│   ├── search/               # planner, fusion (RRF), scorer (recency/importance)
│   ├── organizer/            # async LLM jobs
│   ├── provider/             # embedder / llm implementations (openai/anthropic/gemini/ollama/mock)
│   └── analyzer/             # ja (kagome) / en / bigram
├── interfaces/
│   ├── mcp/                  # MCP server (stdio)
│   └── web/                  # Web UI (build tag: webui)
└── docs/
    └── small-memory-design.md
```

**Placement rules**: New concerns go under the appropriate `internal/<domain>`. `storage.Engine` / `provider.Embedder` / `provider.LLM` / `analyzer.Analyzer` are interface-abstracted, keeping implementations swappable.

---

## 5. Development Commands (Makefile)

The agent should use these by default. Add new targets to the Makefile if needed.

```bash
make build         # go build -o bin/smem ./cmd/smem  (default: pure Go, CGO_ENABLED=0)
make build-all     # cross-compile for all OS/Arch
make run -- <args> # local execution
make test          # go test ./... -race
make test-short    # fast unit tests only
make bench         # benchmarks (search latency, cache hit rate)
make lint          # golangci-lint run
make fmt           # gofumpt / goimports
make tidy          # go mod tidy
make webui         # build with webui tag
```

**The default build uses `CGO_ENABLED=0`.** Features requiring CGO use explicit tags like `-tags treesitter`.

Minimum before PR/commit: `make fmt && make lint && make test`.

---

## 6. Testing and Quality Standards

- **Tests are required** for any behavior-changing modification.
- **Per-layer guidelines**:
  - `storage`: Correctness of WAL replay, crash recovery, compaction, and **LRU eviction** via unit + property tests.
  - `index/fts`: Tokenization (**including Japanese**) and BM25 ranking via golden tests.
  - `index/vector`: Quantization error tolerance and top-k accuracy (exact comparison for flat).
  - `search`: Deterministic RRF fusion and scoring tests.
  - `provider`: Test with `mock` implementation, no network. Real APIs only under `-tags integration`.
- **Always use `-race`.** Concurrency (single writer, lock-free reads) is guarded by the race detector.
- **Determinism**: Tests depending on LLM/embeddings use `mock`; never mix external APIs into unit tests.
- **Regression prevention**: Bug fixes write a reproduction test first, then fix.
- Coverage targets prioritize "critical paths (storage/search) are thick" over numerical thresholds.

---

## 7. Current Milestone (Recommended Order)

> The full roadmap is in design doc §17. **Only the MVP (v0.1) may be implemented now.** Everything else should remain proposals only.

### MVP (v0.1) — "Can Remember"
Recommended implementation order:

1. `smem.Open/Close` with store directory scaffolding and `manifest.json`.
2. `internal/storage`: WAL → memtable → immutable segments → bbolt metadata. Minimum to write and read.
3. Block cache (LRU) and memory budget (**enforce N2 from the start**).
4. `internal/analyzer`: en / bigram → kagome (ja).
5. `index/fts`: Inverted index + BM25.
6. `index/vector`: flat + int8 quantization, cosine/dot.
7. `search`: FTS / Vector / Hybrid (RRF).
8. `provider`: `ollama`, `openai`, `mock` (embedding only is sufficient).
9. `cmd/smem`: `init / add / search / get / rm / ls / stats`.

**MVP Definition of Done**: `init → add (including Japanese content) → search --mode hybrid` works as a single binary (CGO_ENABLED=0), builds for all OSes, and does not exceed the memory budget.

---

## 8. Coding Conventions

- **Errors**: Wrap with `fmt.Errorf("...: %w", err)`. Sentinel errors use `errors.Is/As`. Libraries never panic (except for unrecoverable initialization).
- **context**: External I/O, LLM, and search take `context.Context` as the first argument. Respect cancellation/timeouts.
- **Logging**: `log/slog`. **Never log API keys, full content bodies, or PII.**
- **Minimal public API**: When in doubt, put it in `internal/`. Prioritize backward compatibility of the `smem` package.
- **Interfaces are defined by the consumer**; implementations return concrete types ("accept interfaces, return structs").
- **Concurrency**: Shared state is explicitly protected. Must pass `-race`.
- **Naming/formatting**: gofumpt + goimports. Abbreviations follow conventions (ID, URL, FTS, LRU).
- **File/function size**: Split when things get large. One file, one concern.
- **Comments**: Write the why. Reference design document section numbers for design decisions (e.g., `// see design §6.3 (block cache LRU)`).

---

## 9. Git / PR Workflow

- Branches: `feat/...`, `fix/...`, `refactor/...`, `docs/...`.
- Commits: Conventional Commits (`feat:`, `fix:`, `test:`, `docs:`, `refactor:`, `chore:`).
- **1 PR = 1 concern**. Split large changes.
- PR description template:
  - What and why
  - Which design doc section it corresponds to
  - Impact on non-negotiables (§3)
  - What was tested
  - New dependencies (with rationale if any)
- Pre-merge check: `make fmt lint test` green.

---

## 10. Agent Operating Rules (for Claude Code)

- Follow the sequence **explore → plan → implement → verify**. Never rewrite broad swaths of code without preparation.
- Read related files and tests before making changes. **Match existing conventions.**
- For destructive or wide-ranging operations (bulk file changes, deletions, dependency updates, `go mod` overhauls), **present a diff summary and get confirmation** before executing.
- Run `make test` after completing each task and report results.
- When a design decision is needed, do not decide unilaterally — **present it as a proposed addition to the design doc Decision Log**.
- Do not fill in APIs by guessing. Ask when something is unclear.
- Where benchmarks and measurements are relevant (vector strategy, cache sizing), measure first and then optimize (design doc D4 / Principle 3).

---

## 11. Glossary

| Term | Meaning |
|---|---|
| Collection | Namespace per agent/project. Analyzer, embedding model, and dimensions are fixed. |
| Memory | A memory record (content + metadata + type + importance). |
| Chunk | The unit of search/vector. A segment of a Memory. |
| Segment | An immutable index unit (LSM-style). Merged during compaction. |
| Block Cache | LRU memory cache for segment data blocks (with budget cap). |
| RRF | Reciprocal Rank Fusion. The default fusion method for hybrid search. |
| Tombstone | A logical deletion marker. Physical removal occurs during compaction. |
| Organizer | Async LLM-driven organization jobs (extraction/tagging/consolidation/forgetting). |
| Forgetting | Semantic forgetting (retiring low-value memories). A **distinct concept** from index LRU eviction. |

---

## 12. Known Issues (Decisions Needed During Implementation)

See design doc §18. In particular:
- Vector memory budget (1M × 768 in int8 is ~768MB) → PQ / collection splitting depending on scale.
- tree-sitter CGO boundary (code-aware chunking should be made optional).
- Destruction risk of LLM auto-organization (default soft deletion + dry run).

When implementing features that touch these areas, update the relevant design doc section or confirm the decision.
