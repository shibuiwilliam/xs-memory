# xs-memory

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
go install github.com/xs-memory/xs-memory/cmd/xsmem@latest
```

Make sure `$(go env GOPATH)/bin` is in your `PATH`:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

### Install from source

```bash
git clone https://github.com/xs-memory/xs-memory.git
cd xs-memory
make build
```

The binary lands in `bin/xsmem`. To make it globally available:

```bash
sudo cp bin/xsmem /usr/local/bin/
```

### Basic usage

```bash
# Create a new memory store
xsmem init myproject.xsmem --collection default --analyzer ja

# Add memories
xsmem add myproject.xsmem --collection default notes.md
echo "Claude Codeはターミナルで動くAIアシスタントです" | xsmem add myproject.xsmem --collection default -

# Search
xsmem search myproject.xsmem "AIアシスタント" --mode hybrid --topk 5

# Inspect
xsmem ls myproject.xsmem --collection default
xsmem get myproject.xsmem <memory-id>
xsmem stats myproject.xsmem

# Delete (soft by default)
xsmem rm myproject.xsmem <memory-id>
```

### MCP server

Expose the memory store to AI agents via [Model Context Protocol](https://modelcontextprotocol.io):

```bash
xsmem mcp myproject.xsmem
```

This starts an MCP server over stdio, compatible with Claude Code, VS Code, and other MCP clients.

## CLI Reference

### Global flags

| Flag | Short | Default | Description |
|---|---|---|---|
| `--store <path>` | | `default.xsmem` | Path to store directory |
| `--collection <name>` | `-c` | `default` | Collection name |

### `xsmem init`

Initialize a new store.

```bash
xsmem init [store] [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--analyzer <type>` | `en` | Analyzer type (`en`, `ja`, `bigram`) |
| `--embedder <id>` | | Embedder ID (e.g. `ollama:nomic-embed-text`) |
| `--dim <int>` | `0` | Embedding dimension |

### `xsmem add`

Add memories to the store. Reads from files or stdin (`-`).

```bash
xsmem add [store] [file... | -] [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--type <type>` | `semantic` | Memory type (`episodic`, `semantic`, `procedural`) |
| `--source <source>` | | Source identifier (auto-detected from filename or `stdin`) |
| `--importance <float>` | `0.5` | Importance score (`0..1`) |

### `xsmem search`

Search memories using full-text, vector, or hybrid mode.

```bash
xsmem search [store] <query> [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--mode <mode>` | `hybrid` | Search mode (`fts`, `vector`, `hybrid`) |
| `--topk <int>` | `10` | Number of results to return |
| `--json` | `false` | Output as JSON |

### `xsmem get`

Retrieve a memory by ID.

```bash
xsmem get <id> [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--json` | `false` | Output as JSON |

### `xsmem update`

Update fields of an existing memory. Only explicitly provided flags are applied.

```bash
xsmem update <id> [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--content <text>` | | New content body |
| `--importance <float>` | | New importance score (`0..1`) |
| `--type <type>` | | New memory type (`episodic`, `semantic`, `procedural`) |

### `xsmem rm`

Remove a memory. Soft delete (tombstone) by default.

```bash
xsmem rm <id> [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--hard` | `false` | Permanently delete instead of tombstoning |

### `xsmem ls`

List memories in a collection.

```bash
xsmem ls [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--json` | `false` | Output as JSON |

### `xsmem link`

Create a graph edge (triple) between entities.

```bash
xsmem link <subject> <predicate> <object>
```

No additional flags.

### `xsmem organize`

Run LLM organization jobs manually.

```bash
xsmem organize [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--jobs <list>` | | Comma-separated jobs to run (`extract`, `autotag`, `importance`, `dedup`) |

### `xsmem stats`

Show store statistics (memory count, segments, cache hit rate).

```bash
xsmem stats [flags]
```

| Flag | Default | Description |
|---|---|---|
| `--json` | `false` | Output as JSON |

### `xsmem mcp`

Start an MCP (Model Context Protocol) server over stdio.

```bash
xsmem mcp
```

No additional flags.

### `xsmem export` / `xsmem import`

Export a store to or import from a tar.gz archive.

```bash
xsmem export <archive.tar.gz>
xsmem import <archive.tar.gz>
```

No additional flags.

## Architecture

```
CLI / MCP / Web UI          ← thin adapters
        │
    package xsmem             ← public API (the only stable surface)
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

All interfaces (`CLI`, `MCP`, `Web UI`) talk only to `package xsmem`. Nothing reaches into `internal/` directly.

## Configuration

Settings are loaded with priority: **flags > env vars > config file (TOML) > defaults**.

```toml
[store]
path = "~/.xsmem/default.xsmem"

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

See [PROJECT.md](PROJECT.md) for the full roadmap and non-negotiable design rules, and [docs/xs-memory-design.md](docs/xs-memory-design.md) for the detailed design document.

## License

[MIT](LICENSE)
