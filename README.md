# small-memory

An embedded memory engine for local AI agents — think **"SQLite for AI Agents"**.

Pure Go, single binary, no external servers. Store memories, search them with full-text (BM25), vector, or hybrid search, and optionally let an LLM organize and consolidate knowledge over time.

## Features

- **Embedded-first**: `Open(path)` and go. No separate database process needed.
- **Hybrid search**: BM25 full-text, vector similarity (flat + int8 quantization), and hybrid (Reciprocal Rank Fusion).
- **CJK-ready**: Japanese morphological analysis via [Kagome](https://github.com/ikawaha/kagome) built in from day one.
- **Knowledge graph**: Small-scale triple store for entity relationships.
- **Agent-aware scoring**: Recency, importance, and relevance combine for smarter recall.
- **LLM optional**: Plug in OpenAI, Claude, Gemini, or Ollama for embeddings and async organization — or skip it entirely.
- **Memory budget**: Block-cache LRU keeps RAM usage under a configurable limit (default 256 MB).
- **Cross-platform**: Builds for Linux, macOS, and Windows on amd64 and arm64.

## Quick start

### Install with `go install`

```bash
go install github.com/small-memory/small-memory/cmd/smem@latest
```

Make sure `$(go env GOPATH)/bin` is in your `PATH`:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

### Install from source

```bash
git clone https://github.com/small-memory/small-memory.git
cd small-memory
make build
```

The binary lands in `bin/smem`. To make it globally available:

```bash
sudo cp bin/smem /usr/local/bin/
```

### Basic usage

```bash
# Create a new memory store
smem init myproject.smem --collection default --analyzer ja

# Add memories
smem add myproject.smem --collection default notes.md
echo "Claude Codeはターミナルで動くAIアシスタントです" | smem add myproject.smem --collection default -

# Search
smem search myproject.smem "AIアシスタント" --mode hybrid --topk 5

# Inspect
smem ls myproject.smem --collection default
smem get myproject.smem <memory-id>
smem stats myproject.smem

# Delete (soft by default)
smem rm myproject.smem <memory-id>
```

### MCP server

Expose the memory store to AI agents via [Model Context Protocol](https://modelcontextprotocol.io):

```bash
smem mcp myproject.smem
```

This starts an MCP server over stdio, compatible with Claude Code, VS Code, and other MCP clients.

## CLI Reference

### Global flags

| Flag | Short | Default | Description |
|---|---|---|---|
| `--store <path>` | | `default.smem` | Path to store directory |
| `--collection <name>` | `-c` | `default` | Collection name |

### `smem init`

Initialize a new store.

```bash
smem init [store] [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--analyzer <type>` | `en` | Analyzer type (`en`, `ja`, `bigram`) |
| `--embedder <id>` | | Embedder ID (e.g. `ollama:nomic-embed-text`) |
| `--dim <int>` | `0` | Embedding dimension |

### `smem add`

Add memories to the store. Reads from files or stdin (`-`).

```bash
smem add [store] [file... | -] [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--type <type>` | `semantic` | Memory type (`episodic`, `semantic`, `procedural`) |
| `--source <source>` | | Source identifier (auto-detected from filename or `stdin`) |
| `--importance <float>` | `0.5` | Importance score (`0..1`) |

### `smem search`

Search memories using full-text, vector, or hybrid mode.

```bash
smem search [store] <query> [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--mode <mode>` | `hybrid` | Search mode (`fts`, `vector`, `hybrid`) |
| `--topk <int>` | `10` | Number of results to return |
| `--json` | `false` | Output as JSON |

### `smem get`

Retrieve a memory by ID.

```bash
smem get <id> [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--json` | `false` | Output as JSON |

### `smem update`

Update fields of an existing memory. Only explicitly provided flags are applied.

```bash
smem update <id> [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--content <text>` | | New content body |
| `--importance <float>` | | New importance score (`0..1`) |
| `--type <type>` | | New memory type (`episodic`, `semantic`, `procedural`) |

### `smem rm`

Remove a memory. Soft delete (tombstone) by default.

```bash
smem rm <id> [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--hard` | `false` | Permanently delete instead of tombstoning |

### `smem ls`

List memories in a collection.

```bash
smem ls [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--json` | `false` | Output as JSON |

### `smem link`

Create a graph edge (triple) between entities.

```bash
smem link <subject> <predicate> <object>
```

No additional flags.

### `smem organize`

Run LLM organization jobs manually.

```bash
smem organize [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--jobs <list>` | | Comma-separated jobs to run (`extract`, `autotag`, `importance`, `dedup`) |

### `smem stats`

Show store statistics (memory count, segments, cache hit rate).

```bash
smem stats [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--json` | `false` | Output as JSON |

### `smem mcp`

Start an MCP (Model Context Protocol) server over stdio.

```bash
smem mcp
```

No additional flags.

### `smem export` / `smem import`

Export a store to or import from a tar.gz archive.

```bash
smem export <archive.tar.gz>
smem import <archive.tar.gz>
```

No additional flags.

## Architecture

```
CLI / MCP / Web UI          ← thin adapters
        │
    package smem             ← public API (the only stable surface)
        │
    internal/                ← engine internals
    ├── storage/             WAL, segments, block cache (LRU), bbolt
    ├── index/fts/           inverted index, BM25, analyzers
    ├── index/vector/        flat search, int8 quantization
    ├── index/graph/         triple store (SPO/POS/OSP)
    ├── search/              query planner, RRF fusion, scoring
    ├── ingest/              parse, chunk, embed pipeline
    ├── organizer/           async LLM jobs (extract, tag, summarize, forget)
    ├── provider/            embedder & LLM implementations
    └── analyzer/            ja (Kagome), en, bigram
```

All interfaces (`CLI`, `MCP`, `Web UI`) talk only to `package smem`. Nothing reaches into `internal/` directly.

## Configuration

Settings are loaded with priority: **flags > env vars > config file (TOML) > defaults**.

```toml
[store]
path = "~/.smem/default.smem"

[memory]
block_cache_mb = 256

[embedder]
provider = "ollama"
model    = "nomic-embed-text"

[llm]
provider = "ollama"
model    = "llama3.1"

[ingest]
chunk_tokens = 512
analyzer = "ja"            # ja | en | bigram

[search]
fusion = "rrf"
```

## Development

```bash
make build         # Pure Go build (CGO_ENABLED=0)
make test          # go test ./... -race
make lint          # golangci-lint
make fmt           # gofumpt + goimports
make bench         # Search latency & cache hit benchmarks
make build-all     # Cross-compile for all platforms
```

Always run `make fmt && make lint && make test` before committing.

## Status

**v0.1 (MVP)** — in active development. The goal: `init` → `add` (including Japanese text) → `search --mode hybrid` works in a single pure-Go binary within memory budget.

See [PROJECT.md](PROJECT.md) for the full roadmap and non-negotiable design rules, and [docs/small-memory-design.md](docs/small-memory-design.md) for the detailed design document.

## License

[MIT](LICENSE)
