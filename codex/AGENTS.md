# xs-memory Agent Instructions

This project uses xs-memory as a persistent memory engine via MCP.
The MCP server provides tools for storing, recalling, and organizing memories.

## Setup

The xs-memory MCP server should be registered in your MCP configuration:

```json
{
  "mcpServers": {
    "xs-memory": {
      "command": "xsmem",
      "args": ["mcp", "--store", "~/.xsmem/default.xsmem"]
    }
  }
}
```

## Usage

When the user references past decisions, context, or asks to remember something:

1. **Recall**: Use `memory_recall(query="...", mode="hybrid")` to find relevant memories.
   Answer grounded in the returned memories. Cite memory IDs.

2. **Remember**: Use `memory_store(content="...")` to save new information.
   Apply tags with `memory_set_tags(id, tags="tag1,tag2")`.

3. **Organize**: Call `memory_suggest_organization()` to get a work packet,
   then walk it using the write tools (`memory_merge`, `memory_set_tags`, `memory_link`).

4. **Forget**: Always ask for explicit user confirmation before deleting.
   Use soft delete by default. Hard delete only when explicitly requested with `confirmed=true`.

## Key principle

Retrieval is deterministic (no LLM needed). You (the host model) do all judgment:
summaries, merges, entity extraction. Never spin up a second model.
