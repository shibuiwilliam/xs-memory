// Package mcp implements the MCP (Model Context Protocol) server for xs-memory.
// Uses stdio transport by default. See design §13.2 and addendum §4.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/xs-memory/xs-memory/internal/mcputil"
	"github.com/xs-memory/xs-memory/internal/tuning"
	"github.com/xs-memory/xs-memory/xsmem"
)

// Server wraps an MCP server backed by a xsmem.Store.
// See design §13.2 and addendum §3.
type Server struct {
	store     *xsmem.Store
	mcpServer *server.MCPServer
	logger    *slog.Logger

	// Client info from initialize handshake. See addendum §3.1.
	clientInfo *mcputil.ClientInfo

	// LLM mode resolver config. See addendum §3.1.
	resolverCfg mcputil.ResolverConfig

	// Whether a server-side LLM provider is configured.
	providerConfigured bool
}

// ServerOption configures a Server.
type ServerOption func(*Server)

// WithProviderConfigured marks that a server-side LLM provider is available (Tier 3).
func WithProviderConfigured(v bool) ServerOption {
	return func(s *Server) {
		s.providerConfigured = v
		s.resolverCfg.ProviderConfigured = v
	}
}

// NewServer creates a new MCP server. See design §13.2 and addendum §4.
func NewServer(store *xsmem.Store, opts ...ServerOption) *Server {
	s := &Server{
		store:  store,
		logger: slog.Default(),
	}
	for _, o := range opts {
		o(s)
	}

	mcpSrv := server.NewMCPServer(
		"xs-memory",
		"0.2.0",
		server.WithToolCapabilities(true),
	)

	// --- Existing data tools (design §13.2) ---

	mcpSrv.AddTool(mcp.NewTool(
		"memory_store",
		mcp.WithDescription("Store a new memory"),
		mcp.WithString("content", mcp.Required(), mcp.Description("Memory content")),
		mcp.WithString("collection", mcp.Description("Collection name (default: default)")),
		mcp.WithString("type", mcp.Description("Memory type: episodic, semantic, procedural")),
		mcp.WithString("source", mcp.Description("Source identifier")),
		mcp.WithNumber("importance", mcp.Description("Importance score 0..1")),
	), s.handleStore)

	mcpSrv.AddTool(mcp.NewTool(
		"memory_search",
		mcp.WithDescription("Search memories"),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query text")),
		mcp.WithString("collection", mcp.Description("Collection name")),
		mcp.WithString("mode", mcp.Description("Search mode: fts, vector, hybrid")),
		mcp.WithInteger("topk", mcp.Description("Number of results")),
	), s.handleSearch)

	mcpSrv.AddTool(mcp.NewTool(
		"memory_get",
		mcp.WithDescription("Get a memory by ID"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Memory ID")),
	), s.handleGet)

	mcpSrv.AddTool(mcp.NewTool(
		"memory_update",
		mcp.WithDescription("Update a memory"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Memory ID")),
		mcp.WithString("content", mcp.Description("New content")),
		mcp.WithNumber("importance", mcp.Description("New importance")),
		mcp.WithString("type", mcp.Description("New type")),
	), s.handleUpdate)

	mcpSrv.AddTool(mcp.NewTool(
		"memory_list",
		mcp.WithDescription("List memories in a collection"),
		mcp.WithString("collection", mcp.Description("Collection name")),
	), s.handleList)

	mcpSrv.AddTool(mcp.NewTool(
		"memory_link",
		mcp.WithDescription("Create a graph edge between entities"),
		mcp.WithString("subject", mcp.Required(), mcp.Description("Subject entity")),
		mcp.WithString("predicate", mcp.Required(), mcp.Description("Relationship type")),
		mcp.WithString("object", mcp.Required(), mcp.Description("Object entity")),
		mcp.WithString("source", mcp.Description("Source memory ID for provenance")),
	), s.handleLink)

	// --- New deterministic tools (addendum §4.1, no LLM) ---

	mcpSrv.AddTool(mcp.NewTool(
		"memory_recall",
		mcp.WithDescription("Recall memories relevant to a query (hybrid search + recency/importance scoring)"),
		mcp.WithString("query", mcp.Required(), mcp.Description("What to recall")),
		mcp.WithString("collection", mcp.Description("Collection name")),
		mcp.WithString("mode", mcp.Description("Search mode: fts, vector, hybrid (default: hybrid)")),
		mcp.WithInteger("topk", mcp.Description("Number of results (default: 10)")),
	), s.handleRecall)

	mcpSrv.AddTool(mcp.NewTool(
		"memory_find_duplicate_candidates",
		mcp.WithDescription("Find clusters of near-duplicate memories by vector similarity (no LLM)"),
		mcp.WithString("collection", mcp.Description("Collection name")),
		mcp.WithNumber("threshold", mcp.Description("Similarity threshold 0..1 (default: 0.85)")),
	), s.handleFindDuplicates)

	mcpSrv.AddTool(mcp.NewTool(
		"memory_suggest_organization",
		mcp.WithDescription("Get a work packet: untagged items, duplicate clusters, episodic clusters. No LLM server-side."),
		mcp.WithString("collection", mcp.Description("Collection name")),
	), s.handleSuggestOrganization)

	mcpSrv.AddTool(mcp.NewTool(
		"memory_stats",
		mcp.WithDescription("Store statistics: counts, segments, cache hit-rate"),
	), s.handleStats)

	// --- Write-back tools for host delegation (addendum §4.2) ---

	mcpSrv.AddTool(mcp.NewTool(
		"memory_set_tags",
		mcp.WithDescription("Set tags on a memory"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Memory ID")),
		mcp.WithString("tags", mcp.Required(), mcp.Description("Comma-separated tags")),
	), s.handleSetTags)

	mcpSrv.AddTool(mcp.NewTool(
		"memory_merge",
		mcp.WithDescription("Merge N memories into one (soft-destructive: originals tombstoned). Requires confirmed=true."),
		mcp.WithString("ids", mcp.Required(), mcp.Description("JSON array of memory IDs to merge")),
		mcp.WithString("summary", mcp.Required(), mcp.Description("Host-model-written merged summary")),
		mcp.WithString("collection", mcp.Description("Collection name")),
		mcp.WithBoolean("confirmed", mcp.Required(), mcp.Description("Must be true — safety gate")),
	), s.handleMerge)

	mcpSrv.AddTool(mcp.NewTool(
		"memory_forget",
		mcp.WithDescription("Forget (delete) a memory. Soft delete by default; hard=true for permanent removal."),
		mcp.WithString("id", mcp.Required(), mcp.Description("Memory ID")),
		mcp.WithBoolean("hard", mcp.Description("Permanently delete (requires confirmed=true)")),
		mcp.WithBoolean("confirmed", mcp.Description("Required for hard delete — safety gate (N7)")),
	), s.handleForget)

	// --- Usage & tuning tools (addendum2 §5) ---

	mcpSrv.AddTool(mcp.NewTool(
		"memory_record_usage",
		mcp.WithDescription("Record which memories were actually used after a recall. Feeds adaptive tuning."),
		mcp.WithString("query_id", mcp.Required(), mcp.Description("Query ID from the recall response")),
		mcp.WithString("memory_ids", mcp.Required(), mcp.Description("JSON array of memory IDs that were used")),
		mcp.WithString("outcome", mcp.Description("Outcome: used, cited, ignored (default: used)")),
	), s.handleRecordUsage)

	mcpSrv.AddTool(mcp.NewTool(
		"memory_feedback",
		mcp.WithDescription("Explicit feedback on a memory's usefulness for a query"),
		mcp.WithString("query_id", mcp.Required(), mcp.Description("Query ID")),
		mcp.WithString("memory_id", mcp.Required(), mcp.Description("Memory ID")),
		mcp.WithBoolean("useful", mcp.Required(), mcp.Description("Was this memory useful?")),
	), s.handleFeedback)

	// --- Autonomous helper (addendum §4.3) ---

	mcpSrv.AddTool(mcp.NewTool(
		"memory_organize",
		mcp.WithDescription("Run organization. In host-delegated mode returns a work packet; in provider mode runs server-side."),
		mcp.WithString("collection", mcp.Description("Collection name")),
	), s.handleOrganize)

	s.mcpServer = mcpSrv
	return s
}

// ServeStdio starts the MCP server on stdio. See design §13.2.
func (s *Server) ServeStdio() error {
	stdio := server.NewStdioServer(s.mcpServer)
	return stdio.Listen(context.Background(), os.Stdin, os.Stdout)
}

// SetClientInfo records client identity from the initialize handshake.
// See addendum §3.1.
func (s *Server) SetClientInfo(name, version string, samplingSupported bool) {
	s.clientInfo = &mcputil.ClientInfo{
		Name:              name,
		Version:           version,
		SamplingSupported: samplingSupported,
	}
}

// ResolveLLMMode resolves the current LLM execution mode.
// See addendum §3.1.
func (s *Server) ResolveLLMMode() mcputil.LLMMode {
	// If we have a client, we're in interactive MCP mode → host-delegated.
	interactive := s.clientInfo != nil
	return mcputil.ResolveLLMMode(s.clientInfo, s.resolverCfg, interactive)
}

// --- Existing tool handlers ---

func (s *Server) handleStore(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	content, _ := req.GetArguments()["content"].(string)
	collection, _ := req.GetArguments()["collection"].(string)
	if collection == "" {
		collection = "default"
	}
	memType, _ := req.GetArguments()["type"].(string)
	if memType == "" {
		memType = "semantic"
	}
	source, _ := req.GetArguments()["source"].(string)
	importance := float32(0.5)
	if v, ok := req.GetArguments()["importance"].(float64); ok {
		importance = float32(v)
	}

	id, err := s.store.Remember(context.Background(), xsmem.RememberOpts{
		Collection:  collection,
		Content:     content,
		ContentType: "text/plain",
		Source:      source,
		Type:        xsmem.MemoryType(memType),
		Importance:  importance,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("store failed: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf(`{"id":"%s"}`, id)), nil
}

func (s *Server) handleSearch(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, _ := req.GetArguments()["query"].(string)
	collection, _ := req.GetArguments()["collection"].(string)
	if collection == "" {
		collection = "default"
	}
	modeStr, _ := req.GetArguments()["mode"].(string)
	topk := 10
	if v, ok := req.GetArguments()["topk"].(float64); ok {
		topk = int(v)
	}

	var mode xsmem.SearchMode
	switch modeStr {
	case "fts":
		mode = xsmem.FTS
	case "vector":
		mode = xsmem.Vector
	default:
		mode = xsmem.Hybrid
	}

	results, err := s.store.Search(context.Background(), xsmem.SearchOpts{
		Collection: collection, Text: query, Mode: mode, TopK: topk,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
	}
	data, _ := json.Marshal(results)
	return mcp.NewToolResultText(string(data)), nil
}

func (s *Server) handleGet(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, _ := req.GetArguments()["id"].(string)
	mem, err := s.store.Get(context.Background(), id)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("get failed: %v", err)), nil
	}
	data, _ := json.Marshal(mem)
	return mcp.NewToolResultText(string(data)), nil
}

func (s *Server) handleUpdate(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, _ := req.GetArguments()["id"].(string)
	patch := xsmem.UpdateOpts{}
	if c, ok := req.GetArguments()["content"].(string); ok {
		patch.Content = &c
	}
	if v, ok := req.GetArguments()["importance"].(float64); ok {
		f := float32(v)
		patch.Importance = &f
	}
	if t, ok := req.GetArguments()["type"].(string); ok {
		mt := xsmem.MemoryType(t)
		patch.Type = &mt
	}
	if err := s.store.Update(context.Background(), id, patch); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("update failed: %v", err)), nil
	}
	return mcp.NewToolResultText(`{"status":"updated"}`), nil
}

func (s *Server) handleList(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	collection, _ := req.GetArguments()["collection"].(string)
	if collection == "" {
		collection = "default"
	}
	mems, err := s.store.List(context.Background(), collection)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("list failed: %v", err)), nil
	}
	data, _ := json.Marshal(mems)
	return mcp.NewToolResultText(string(data)), nil
}

func (s *Server) handleLink(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	subject, _ := req.GetArguments()["subject"].(string)
	predicate, _ := req.GetArguments()["predicate"].(string)
	object, _ := req.GetArguments()["object"].(string)
	source, _ := req.GetArguments()["source"].(string)

	err := s.store.Link(context.Background(), xsmem.Triple{
		Subject: subject, Predicate: predicate, Object: object,
		Weight: 1.0, Source: source,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("link failed: %v", err)), nil
	}
	return mcp.NewToolResultText(`{"status":"linked"}`), nil
}

// --- New deterministic tool handlers (addendum §4.1) ---

func (s *Server) handleRecall(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Convenience wrapper: hybrid search tuned for agent recall. See addendum §4.1.
	query, _ := req.GetArguments()["query"].(string)
	collection, _ := req.GetArguments()["collection"].(string)
	if collection == "" {
		collection = "default"
	}
	modeStr, _ := req.GetArguments()["mode"].(string)
	topk := 10
	if v, ok := req.GetArguments()["topk"].(float64); ok {
		topk = int(v)
	}

	var mode xsmem.SearchMode
	switch modeStr {
	case "fts":
		mode = xsmem.FTS
	case "vector":
		mode = xsmem.Vector
	default:
		mode = xsmem.Hybrid
	}

	results, err := s.store.Search(context.Background(), xsmem.SearchOpts{
		Collection: collection, Text: query, Mode: mode, TopK: topk,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("recall failed: %v", err)), nil
	}
	data, _ := json.Marshal(results)
	return mcp.NewToolResultText(string(data)), nil
}

func (s *Server) handleFindDuplicates(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	collection, _ := req.GetArguments()["collection"].(string)
	threshold := 0.85
	if v, ok := req.GetArguments()["threshold"].(float64); ok {
		threshold = v
	}

	clusters, err := s.store.FindDuplicateCandidates(context.Background(), collection, threshold)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("find duplicates failed: %v", err)), nil
	}
	data, _ := json.Marshal(clusters)
	return mcp.NewToolResultText(string(data)), nil
}

func (s *Server) handleSuggestOrganization(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	collection, _ := req.GetArguments()["collection"].(string)

	wp, err := s.store.SuggestOrganization(context.Background(), collection)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("suggest organization failed: %v", err)), nil
	}
	data, _ := json.Marshal(wp)
	return mcp.NewToolResultText(string(data)), nil
}

func (s *Server) handleStats(_ context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	stats := s.store.Stats()
	data, _ := json.Marshal(stats)
	return mcp.NewToolResultText(string(data)), nil
}

// --- Write-back tool handlers (addendum §4.2) ---

func (s *Server) handleSetTags(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, _ := req.GetArguments()["id"].(string)
	tagsStr, _ := req.GetArguments()["tags"].(string)

	// Parse comma-separated tags into a slice.
	var tags []string
	for _, t := range splitTags(tagsStr) {
		if t != "" {
			tags = append(tags, t)
		}
	}

	err := s.store.Update(context.Background(), id, xsmem.UpdateOpts{
		Metadata: map[string]any{"tags": tags},
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("set_tags failed: %v", err)), nil
	}
	return mcp.NewToolResultText(`{"status":"tags_set"}`), nil
}

func (s *Server) handleMerge(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	idsJSON, _ := req.GetArguments()["ids"].(string)
	summary, _ := req.GetArguments()["summary"].(string)
	collection, _ := req.GetArguments()["collection"].(string)
	confirmed, _ := req.GetArguments()["confirmed"].(bool)

	var ids []string
	if err := json.Unmarshal([]byte(idsJSON), &ids); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid ids JSON: %v", err)), nil
	}

	newID, err := s.store.Merge(context.Background(), xsmem.MergeOpts{
		Collection: collection,
		IDs:        ids,
		Summary:    summary,
		Confirmed:  confirmed,
	})
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("merge failed: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf(`{"id":"%s","status":"merged"}`, newID)), nil
}

func (s *Server) handleForget(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	id, _ := req.GetArguments()["id"].(string)
	hard, _ := req.GetArguments()["hard"].(bool)
	confirmed, _ := req.GetArguments()["confirmed"].(bool)

	// Hard delete requires explicit confirmation (N7, H5).
	if hard && !confirmed {
		return mcp.NewToolResultError("hard delete requires confirmed=true (design N7, addendum H5)"), nil
	}

	if err := s.store.Forget(context.Background(), id, hard); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("forget failed: %v", err)), nil
	}

	kind := "soft"
	if hard {
		kind = "hard"
	}
	return mcp.NewToolResultText(fmt.Sprintf(`{"status":"deleted","kind":"%s"}`, kind)), nil
}

// --- Autonomous helper (addendum §4.3) ---

func (s *Server) handleOrganize(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	collection, _ := req.GetArguments()["collection"].(string)
	mode := s.ResolveLLMMode()

	switch mode {
	case mcputil.ModeProvider:
		// Tier 3: run organizer server-side.
		err := s.store.Organize(context.Background(), collection)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("organize failed: %v", err)), nil
		}
		return mcp.NewToolResultText(`{"status":"organized","mode":"provider"}`), nil

	case mcputil.ModeHostDelegated, mcputil.ModeSampling, mcputil.ModeDisabled:
		// Tier 1, 2, or 4: return work packet instead of calling LLM.
		// See addendum §4.3, H7: degrades to work packet, never fails.
		wp, err := s.store.SuggestOrganization(context.Background(), collection)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("suggest organization failed: %v", err)), nil
		}
		result := map[string]any{
			"mode":        mode.String(),
			"work_packet": wp,
			"hint":        "No server-side model available in this mode. Drive organization via the host agent using the write tools.",
		}
		data, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(data)), nil

	default:
		return mcp.NewToolResultText(`{"status":"queued","mode":"disabled"}`), nil
	}
}

// --- Usage & tuning tool handlers (addendum2 §5) ---

func (s *Server) handleRecordUsage(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	queryID, _ := req.GetArguments()["query_id"].(string)
	idsJSON, _ := req.GetArguments()["memory_ids"].(string)
	outcome, _ := req.GetArguments()["outcome"].(string)
	if outcome == "" {
		outcome = "used"
	}

	var memIDs []string
	if err := json.Unmarshal([]byte(idsJSON), &memIDs); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("invalid memory_ids JSON: %v", err)), nil
	}

	s.store.RecordUsage(tuning.UsageEvent{
		QueryID:     queryID,
		Impressions: memIDs, // all IDs were at least impressed
		Used:        memIDs, // reported as used
	})

	return mcp.NewToolResultText(`{"status":"recorded"}`), nil
}

func (s *Server) handleFeedback(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	queryID, _ := req.GetArguments()["query_id"].(string)
	memoryID, _ := req.GetArguments()["memory_id"].(string)
	useful, _ := req.GetArguments()["useful"].(bool)

	var used []string
	if useful {
		used = []string{memoryID}
	}

	s.store.RecordUsage(tuning.UsageEvent{
		QueryID:     queryID,
		Impressions: []string{memoryID},
		Used:        used,
	})

	return mcp.NewToolResultText(`{"status":"feedback_recorded"}`), nil
}

// --- helpers ---

func splitTags(s string) []string {
	var tags []string
	current := ""
	for _, r := range s {
		if r == ',' {
			tags = append(tags, current)
			current = ""
		} else {
			current += string(r)
		}
	}
	if current != "" {
		tags = append(tags, current)
	}
	// Trim whitespace.
	for i := range tags {
		tags[i] = trimSpace(tags[i])
	}
	return tags
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
