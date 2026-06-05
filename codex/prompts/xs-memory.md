# xs-memory

Persistent memory for this project, backed by the xs-memory MCP server.
Retrieval is deterministic and runs in the server. **You (the host model) do
all judgment** — summaries, merges, entity/relation extraction — and persist
results with the write tools. Do not spin up another model.

## Routing

When the user mentions memory, recall, remembering, or past decisions:

- **recall** / questions about past context: call `memory_recall(query="<query>", mode="hybrid")`,
  then answer grounded strictly in the returned memories. Cite memory IDs.
- **remember** / save to memory: call `memory_store(content="<text>")`. Infer a short tag set and
  apply with `memory_set_tags`. Confirm what was stored (one line).
- **organize**: run the organization loop below.
- **forget**: this is destructive. Show the user the exact memories matched and
  ask for explicit confirmation. Only after a clear "yes" should a deletion tool
  be used. Default to soft delete; never hard-delete without an explicit request.

## Organization loop (host-delegated — uses YOUR model, no extra LLM)

1. Call `memory_suggest_organization(collection)` to get a work packet.
2. For each **duplicate cluster**: read the items, decide if they truly say the
   same thing. If yes, write one faithful merged summary and call
   `memory_merge(ids=[…], summary="…", confirmed=true)`. Preserve any unique detail.
   If unsure, skip — do not merge.
3. For each **untagged** item: assign 1–4 concise tags via `memory_set_tags`.
4. For each **unlinked** item: extract concrete entities and relations and add
   them with `memory_link(s, p, o)`. Only assert relations stated in the text.
5. For **episodic clusters** ready to consolidate: write a semantic summary,
   `memory_store` it as type=semantic, and link it to its sources.
6. Report what changed in 3–5 bullets. Never delete during organization.

## Rules

- Ground every recalled claim in a returned memory; if memory is empty, say so.
- Keep stored content faithful to the source; do not invent.
- Merges and deletes are gated: confirm before anything destructive.

## Available tools

`memory_recall`, `memory_search`, `memory_get`, `memory_list`, `memory_store`,
`memory_update`, `memory_set_tags`, `memory_link`, `memory_find_duplicate_candidates`,
`memory_suggest_organization`, `memory_stats`, `memory_merge`, `memory_forget`,
`memory_organize`
