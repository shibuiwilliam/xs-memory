# xs-memory Tool Reference

Full reference for all MCP tools exposed by the xs-memory server.

## Read-only / deterministic tools (no LLM, safe to auto-invoke)

### `memory_recall`
Recall memories relevant to a query. Convenience wrapper over `memory_search` tuned for agent recall.
- `query` (required): What to recall
- `collection`: Collection name (default: "default")
- `mode`: Search mode — `fts`, `vector`, `hybrid` (default: hybrid)
- `topk`: Number of results (default: 10)

### `memory_search`
Low-level search with full control.
- `query` (required): Search query text
- `collection`: Collection name
- `mode`: `fts`, `vector`, `hybrid`
- `topk`: Number of results

### `memory_get`
Retrieve a single memory by ID.
- `id` (required): Memory ULID

### `memory_list`
List all memories in a collection.
- `collection`: Collection name

### `memory_find_duplicate_candidates`
Find clusters of near-duplicate memories by vector similarity. Pure vector math, no LLM.
- `collection`: Collection name
- `threshold`: Similarity threshold 0..1 (default: 0.85)

### `memory_suggest_organization`
Get a work packet: untagged items, duplicate clusters, episodic clusters ready for summarization. No LLM server-side.
- `collection`: Collection name

### `memory_stats`
Store statistics: memory count, collection count, cache usage.

## Write tools (non-destructive)

### `memory_store`
Store a new memory.
- `content` (required): Memory content
- `collection`: Collection name (default: "default")
- `type`: `episodic`, `semantic`, `procedural` (default: semantic)
- `source`: Source identifier
- `importance`: 0..1 (default: 0.5)

### `memory_update`
Update a memory's mutable fields.
- `id` (required): Memory ID
- `content`: New content
- `importance`: New importance score
- `type`: New memory type

### `memory_set_tags`
Set tags on a memory.
- `id` (required): Memory ID
- `tags` (required): Comma-separated tags

### `memory_link`
Create a knowledge graph edge.
- `subject` (required): Subject entity
- `predicate` (required): Relationship type
- `object` (required): Object entity
- `source`: Source memory ID for provenance

## Soft-destructive tools (require confirmation)

### `memory_merge`
Merge N memories into one. Originals are tombstoned (soft deleted).
- `ids` (required): JSON array of memory IDs — e.g., `["id1","id2"]`
- `summary` (required): The merged summary text (written by the host model)
- `collection`: Collection name
- `confirmed` (required): Must be `true` — safety gate

### `memory_forget`
Delete a memory. Soft delete by default.
- `id` (required): Memory ID
- `hard`: Set to `true` for permanent deletion
- `confirmed`: Required when `hard=true` — safety gate

### `memory_organize`
Run organization. In host-delegated mode (default when used from an agent), returns a work packet. In provider mode (headless/CLI), runs server-side.
- `collection`: Collection name

## Memory types

| Type | Description |
|---|---|
| `episodic` | Conversations, events. Recency-weighted. Primary consolidation target. |
| `semantic` | Facts, knowledge. Stable. Promoted from episodic via consolidation. |
| `procedural` | Procedures, skills, how-tos. |
