// Package xsmem is the public API for xs-memory, an embedded memory engine
// for local AI agents. All UI layers (CLI, MCP, Web) use this package exclusively.
// See design §2 (principle 2) and PROJECT.md N3/N9.
package xsmem

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/xs-memory/xs-memory/internal/analyzer"
	icache "github.com/xs-memory/xs-memory/internal/cache"
	"github.com/xs-memory/xs-memory/internal/index/fts"
	"github.com/xs-memory/xs-memory/internal/index/graph"
	"github.com/xs-memory/xs-memory/internal/index/grep"
	"github.com/xs-memory/xs-memory/internal/index/vector"
	"github.com/xs-memory/xs-memory/internal/ingest"
	"github.com/xs-memory/xs-memory/internal/organizer"
	"github.com/xs-memory/xs-memory/internal/provider"
	"github.com/xs-memory/xs-memory/internal/search"
	"github.com/xs-memory/xs-memory/internal/storage"
	"github.com/xs-memory/xs-memory/internal/tuning"
)

// MemoryType re-exports storage.MemoryType for public API.
type MemoryType = storage.MemoryType

const (
	Episodic   MemoryType = storage.MemoryTypeEpisodic
	Semantic   MemoryType = storage.MemoryTypeSemantic
	Procedural MemoryType = storage.MemoryTypeProcedural
)

// SearchMode re-exports search.Mode for public API.
type SearchMode = search.Mode

const (
	FTS    SearchMode = search.ModeFTS
	Vector SearchMode = search.ModeVector
	Hybrid SearchMode = search.ModeHybrid
)

// Memory is the public representation of a memory record.
type Memory struct {
	ID          string         `json:"id"`
	Collection  string         `json:"collection"`
	Content     string         `json:"content"`
	ContentType string         `json:"content_type"`
	Source      string         `json:"source"`
	Type        MemoryType     `json:"type"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Importance  float32        `json:"importance"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	AccessedAt  time.Time      `json:"accessed_at"`
	AccessCount uint32         `json:"access_count"`
}

// Result is a scored search result. See design App. A.
type Result struct {
	Memory    Memory  `json:"memory"`
	Score     float64 `json:"score"`
	ChunkText string  `json:"chunk_text,omitempty"`
}

// Store is the main handle to an xs-memory store directory.
// See design §2 principle 1: SQLite mental model.
type Store struct {
	path     string
	mu       sync.RWMutex
	closed   bool
	cfg      config
	manifest *storage.Manifest
	meta     *storage.MetaStore
	wal      *storage.WAL
	cache    *storage.BlockCache
	lock     *storage.FileLock
	logger   *slog.Logger

	// Per-collection indexes.
	ftsIndexes map[string]*fts.Index
	vecIndexes map[string]*vector.Index
	analyzers  map[string]analyzer.Analyzer

	// Providers.
	embedder provider.Embedder

	// Ingestion.
	pipeline *ingest.Pipeline

	// Knowledge graph. See design §7.3.
	graph *graph.Graph

	// LLM organizer. See design §10.
	organizer *organizer.Organizer

	// Result cache. See addendum2 §2.
	resultCache *icache.ResultCache

	// Grep engine. See addendum2 §3.
	grepEngine *grep.Engine

	// Tuning store. See addendum2 §1.
	tuningStore *tuning.Store
}

// Open opens or creates an xs-memory store at the given path.
// See design §6.1 for file layout.
func Open(path string, opts ...Option) (*Store, error) {
	cfg := defaultConfig()
	for _, o := range opts {
		o(&cfg)
	}

	logger := slog.Default()

	// Ensure the store directory exists.
	if err := os.MkdirAll(path, 0o755); err != nil {
		return nil, fmt.Errorf("xsmem: create store directory: %w", err)
	}

	// Create subdirectories per design §6.1.
	for _, sub := range []string{"wal", "segments", "blobs"} {
		if err := os.MkdirAll(filepath.Join(path, sub), 0o755); err != nil {
			return nil, fmt.Errorf("xsmem: create %s directory: %w", sub, err)
		}
	}

	// File lock for exclusive access. See design §12.
	lock, err := storage.LockStore(path)
	if err != nil {
		return nil, fmt.Errorf("xsmem: %w", err)
	}

	// Read manifest. See design §6.1.
	manifest, err := storage.ReadManifest(path)
	if err != nil {
		lock.Unlock()
		return nil, fmt.Errorf("xsmem: %w", err)
	}

	// Open bbolt meta store.
	meta, err := storage.OpenMeta(filepath.Join(path, "meta.db"))
	if err != nil {
		lock.Unlock()
		return nil, fmt.Errorf("xsmem: %w", err)
	}

	// Open WAL. See design §6.2.
	wal, err := storage.OpenWAL(filepath.Join(path, "wal"))
	if err != nil {
		meta.Close()
		lock.Unlock()
		return nil, fmt.Errorf("xsmem: %w", err)
	}

	// Block cache with memory budget. See design §6.3, N2.
	cache := storage.NewBlockCache(cfg.BlockCacheMB * 1024 * 1024)

	s := &Store{
		path:       path,
		cfg:        cfg,
		manifest:   manifest,
		meta:       meta,
		wal:        wal,
		cache:      cache,
		lock:       lock,
		logger:     logger,
		ftsIndexes: make(map[string]*fts.Index),
		vecIndexes: make(map[string]*vector.Index),
		analyzers:  make(map[string]analyzer.Analyzer),
		embedder:   cfg.Embedder,
	}

	// Set up embedder.
	s.pipeline = ingest.NewPipeline(s.embedder, ingest.Config{
		ChunkTokens:  cfg.ChunkTokens,
		ChunkOverlap: cfg.ChunkOverlap,
	})

	// Initialize knowledge graph. See design §7.3.
	g, err := graph.Open(meta.DB())
	if err != nil {
		logger.Warn("graph init failed", "error", err)
	} else {
		s.graph = g
	}

	// Initialize organizer. See design §10.
	s.initOrganizer()

	// Initialize result cache. See addendum2 §2.
	s.resultCache = icache.NewResultCache(100, 32*1024*1024, 16)

	// Initialize grep engine. See addendum2 §3.
	s.grepEngine = grep.NewEngine()

	// Initialize tuning store. See addendum2 §1.
	s.tuningStore = tuning.NewStore(tuning.DefaultConfig())

	// Replay WAL for crash recovery. See design §6.5.
	if err := s.replayWAL(); err != nil {
		logger.Warn("WAL replay encountered issues", "error", err)
	}

	// Initialize indexes for existing collections.
	for _, col := range manifest.Collections {
		if err := s.initCollectionIndexes(col); err != nil {
			logger.Warn("init collection indexes", "collection", col.Name, "error", err)
		}
	}

	return s, nil
}

// Close flushes pending data and releases all resources. See design §6.5.
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return fmt.Errorf("xsmem: store already closed")
	}
	s.closed = true

	// Save manifest.
	if err := storage.WriteManifest(s.path, s.manifest); err != nil {
		s.logger.Error("write manifest on close", "error", err)
	}

	var firstErr error
	if err := s.wal.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := s.meta.Close(); err != nil && firstErr == nil {
		firstErr = err
	}
	if err := s.lock.Unlock(); err != nil && firstErr == nil {
		firstErr = err
	}
	return firstErr
}

// Path returns the store directory path.
func (s *Store) Path() string {
	return s.path
}

// CreateCollection creates a new collection with the given settings.
// Analyzer and embedding dimension are pinned. See design §5.1, N6.
func (s *Store) CreateCollection(name string, analyzerID string, embedderID string, embedDim int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.manifest.Collections[name]; exists {
		return fmt.Errorf("xsmem: collection %q already exists", name)
	}

	col := storage.CollectionConfig{
		Name:           name,
		Analyzer:       analyzerID,
		EmbedderID:     embedderID,
		EmbedDimension: embedDim,
	}

	s.manifest.Collections[name] = col
	if err := storage.WriteManifest(s.path, s.manifest); err != nil {
		return fmt.Errorf("xsmem: save manifest: %w", err)
	}

	if err := s.meta.PutCollection(col); err != nil {
		return fmt.Errorf("xsmem: save collection meta: %w", err)
	}

	return s.initCollectionIndexes(col)
}

// Remember stores a new memory. See design App. A.
func (s *Store) Remember(ctx context.Context, opts RememberOpts) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return "", fmt.Errorf("xsmem: store closed")
	}

	// Verify collection exists; auto-create "default" if needed.
	col, ok := s.manifest.Collections[opts.Collection]
	if !ok {
		if opts.Collection == "" || opts.Collection == "default" {
			opts.Collection = "default"
			analyzerID := "en"
			if s.cfg.DefaultAnalyzer != "" {
				analyzerID = s.cfg.DefaultAnalyzer
			}
			embedDim := 0
			embedderID := ""
			if s.embedder != nil {
				embedDim = s.embedder.Dim()
				embedderID = s.embedder.ID()
			}
			col = storage.CollectionConfig{
				Name: "default", Analyzer: analyzerID,
				EmbedderID: embedderID, EmbedDimension: embedDim,
			}
			s.manifest.Collections["default"] = col
			storage.WriteManifest(s.path, s.manifest)
			s.meta.PutCollection(col)
			s.initCollectionIndexes(col)
		} else {
			return "", fmt.Errorf("xsmem: collection %q not found", opts.Collection)
		}
	}

	id, err := ulid.New(ulid.Now(), rand.Reader)
	if err != nil {
		return "", fmt.Errorf("xsmem: generate id: %w", err)
	}

	// Run ingestion pipeline.
	result, err := s.pipeline.Ingest(ctx, ingest.IngestOpts{
		ID:          id,
		Collection:  opts.Collection,
		Content:     opts.Content,
		ContentType: opts.ContentType,
		Source:      opts.Source,
		Type:        opts.Type,
		Metadata:    opts.Metadata,
		Importance:  opts.Importance,
	})
	if err != nil {
		return "", fmt.Errorf("xsmem: ingest: %w", err)
	}

	// Write to WAL. See design §6.2.
	memJSON, err := json.Marshal(result.Memory)
	if err != nil {
		return "", fmt.Errorf("xsmem: marshal memory: %w", err)
	}
	if err := s.wal.Append(storage.WALRecord{
		Op: storage.WALOpPut, MemoryID: id, Data: memJSON,
	}); err != nil {
		return "", fmt.Errorf("xsmem: wal append: %w", err)
	}

	// Store in meta.
	if err := s.meta.PutMemory(result.Memory); err != nil {
		return "", fmt.Errorf("xsmem: store meta: %w", err)
	}

	// Index chunks.
	s.indexChunks(opts.Collection, id, result.Chunks)

	// Invalidate result cache for this collection. See addendum2 §2.3.
	s.resultCache.BumpGeneration(opts.Collection)

	s.logger.Debug("memory stored", "id", id.String(), "collection", opts.Collection,
		"chunks", len(result.Chunks))

	return id.String(), nil
}

// RememberOpts are the options for Remember.
type RememberOpts struct {
	Collection  string
	Content     string
	ContentType string
	Source      string
	Type        MemoryType
	Metadata    map[string]any
	Importance  float32
}

// Search searches for memories. See design App. A.
func (s *Store) Search(ctx context.Context, opts SearchOpts) ([]Result, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, fmt.Errorf("xsmem: store closed")
	}

	col := opts.Collection
	if col == "" {
		col = "default"
	}

	ftsIdx := s.ftsIndexes[col]
	vecIdx := s.vecIndexes[col]

	searcher := search.NewSearcher(ftsIdx, vecIdx, s.embedder)

	results, err := searcher.Search(ctx, search.Query{
		Collection: col,
		Text:       opts.Text,
		Vector:     opts.Vector,
		Mode:       opts.Mode,
		TopK:       opts.TopK,
		RRFk:       60,
	})
	if err != nil {
		return nil, fmt.Errorf("xsmem: search: %w", err)
	}

	// Convert to public results, resolving memory metadata.
	out := make([]Result, 0, len(results))
	for _, r := range results {
		mem, err := s.meta.GetMemory(r.MemoryID)
		if err != nil {
			continue // tombstoned or missing
		}
		if mem.Deleted {
			continue
		}
		out = append(out, Result{
			Memory:    toPublicMemory(mem),
			Score:     r.Score,
			ChunkText: r.ChunkText,
		})
	}

	return out, nil
}

// SearchOpts are the options for Search.
type SearchOpts struct {
	Collection string
	Text       string
	Vector     []float32
	Mode       SearchMode
	TopK       int
	// Grep options. See addendum2 §3.
	GrepEnabled  bool
	GrepPattern  string
	GrepRegex    bool
	GrepCaseSens bool
}

// GrepResult holds grep-specific output. See addendum2 §3.3.
type GrepResult struct {
	Truncated  bool `json:"grep_truncated,omitempty"`
	Highlights map[string][]struct {
		Start int `json:"start"`
		End   int `json:"end"`
	} `json:"highlights,omitempty"`
}

// Get retrieves a memory by ID. See design App. A.
func (s *Store) Get(ctx context.Context, id string) (*Memory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	uid, err := ulid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("xsmem: invalid id: %w", err)
	}

	mem, err := s.meta.GetMemory(uid)
	if err != nil {
		return nil, fmt.Errorf("xsmem: %w", err)
	}
	if mem.Deleted {
		return nil, fmt.Errorf("xsmem: memory %s is deleted", id)
	}

	// Update access stats.
	mem.AccessedAt = time.Now()
	mem.AccessCount++
	s.meta.PutMemory(mem)

	pub := toPublicMemory(mem)
	return &pub, nil
}

// Update updates a memory's mutable fields. See design App. A.
func (s *Store) Update(ctx context.Context, id string, patch UpdateOpts) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	uid, err := ulid.Parse(id)
	if err != nil {
		return fmt.Errorf("xsmem: invalid id: %w", err)
	}

	mem, err := s.meta.GetMemory(uid)
	if err != nil {
		return fmt.Errorf("xsmem: %w", err)
	}
	if mem.Deleted {
		return fmt.Errorf("xsmem: memory %s is deleted", id)
	}

	if patch.Content != nil {
		mem.Content = *patch.Content
	}
	if patch.Importance != nil {
		mem.Importance = *patch.Importance
	}
	if patch.Type != nil {
		mem.Type = *patch.Type
	}
	if patch.Metadata != nil {
		mem.Metadata = patch.Metadata
	}
	mem.UpdatedAt = time.Now()

	// Invalidate cache. See addendum2 §2.3.
	s.resultCache.BumpGeneration(mem.Collection)

	// Re-index if content changed.
	if patch.Content != nil {
		// Re-ingest.
		result, err := s.pipeline.Ingest(ctx, ingest.IngestOpts{
			ID: uid, Collection: mem.Collection, Content: *patch.Content,
			ContentType: mem.ContentType, Source: mem.Source,
			Type: mem.Type, Metadata: mem.Metadata, Importance: mem.Importance,
		})
		if err == nil {
			s.indexChunks(mem.Collection, uid, result.Chunks)
		}
	}

	return s.meta.PutMemory(mem)
}

// UpdateOpts are the mutable fields for Update.
type UpdateOpts struct {
	Content    *string
	Importance *float32
	Type       *MemoryType
	Metadata   map[string]any
}

// Forget deletes a memory. Soft by default (N7). See design App. A.
func (s *Store) Forget(ctx context.Context, id string, hard bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	uid, err := ulid.Parse(id)
	if err != nil {
		return fmt.Errorf("xsmem: invalid id: %w", err)
	}

	// Write to WAL.
	if err := s.wal.Append(storage.WALRecord{
		Op: storage.WALOpDelete, MemoryID: uid,
	}); err != nil {
		return fmt.Errorf("xsmem: wal append: %w", err)
	}

	// Get memory to know its collection (for index removal).
	mem, err := s.meta.GetMemory(uid)
	if err == nil && !mem.Deleted {
		if idx, ok := s.ftsIndexes[mem.Collection]; ok {
			idx.Remove(uid)
		}
		if idx, ok := s.vecIndexes[mem.Collection]; ok {
			idx.Remove(uid)
		}
		// Invalidate cache and purge tuning signals. See addendum2 §2.3, §1.5.
		s.resultCache.BumpGeneration(mem.Collection)
		s.tuningStore.PurgeItem(mem.Collection, id)
	}

	return s.meta.DeleteMemory(uid, hard)
}

// List lists memories in a collection.
func (s *Store) List(ctx context.Context, collection string) ([]Memory, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	mems, err := s.meta.ListMemories(collection)
	if err != nil {
		return nil, fmt.Errorf("xsmem: %w", err)
	}

	result := make([]Memory, len(mems))
	for i, m := range mems {
		result[i] = toPublicMemory(m)
	}
	return result, nil
}

// Stats returns store statistics.
type Stats struct {
	Path            string             `json:"path"`
	Collections     int                `json:"collections"`
	Memories        int                `json:"memories"`
	BlockCacheStats CacheStatsInfo     `json:"block_cache"`
	ResultCache     icache.Stats       `json:"result_cache"`
	Tuning          tuning.TuningStats `json:"tuning"`
}

type CacheStatsInfo struct {
	CapacityMB float64 `json:"capacity_mb"`
	UsedMB     float64 `json:"used_mb"`
	Count      int     `json:"count"`
}

func (s *Store) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cs := s.cache.Stats()
	mems, _ := s.meta.ListMemories("")

	return Stats{
		Path:        s.path,
		Collections: len(s.manifest.Collections),
		Memories:    len(mems),
		BlockCacheStats: CacheStatsInfo{
			CapacityMB: float64(cs.Capacity) / (1024 * 1024),
			UsedMB:     float64(cs.Used) / (1024 * 1024),
			Count:      cs.Count,
		},
		ResultCache: s.resultCache.Stats(),
		Tuning:      s.tuningStore.Stats(),
	}
}

// RecordUsage records a usage event for adaptive tuning. See addendum2 §1.1.
func (s *Store) RecordUsage(event tuning.UsageEvent) {
	s.tuningStore.RecordUsage(event)
}

// TuningReset clears all learned signals. See addendum2 §1.5.
func (s *Store) TuningReset(collection string) {
	s.tuningStore.Reset(collection)
}

// TuningEpoch returns the current tuning epoch.
func (s *Store) TuningEpoch() uint64 {
	return s.tuningStore.Epoch()
}

// --- internal helpers ---

func (s *Store) initCollectionIndexes(col storage.CollectionConfig) error {
	// Initialize analyzer.
	a, err := analyzer.New(col.Analyzer)
	if err != nil {
		return fmt.Errorf("init analyzer %q: %w", col.Analyzer, err)
	}
	s.analyzers[col.Name] = a

	// Create FTS index.
	s.ftsIndexes[col.Name] = fts.NewIndex(a)

	// Create vector index if embedding is configured.
	if col.EmbedDimension > 0 {
		s.vecIndexes[col.Name] = vector.NewIndex(col.EmbedDimension, vector.Cosine, true)
	}

	// Re-index existing memories from meta store.
	mems, err := s.meta.ListMemories(col.Name)
	if err != nil {
		return nil // no memories yet
	}
	for _, mem := range mems {
		if ftsIdx, ok := s.ftsIndexes[col.Name]; ok {
			ftsIdx.Add(mem.ID, mem.Content)
		}
	}

	return nil
}

func (s *Store) indexChunks(collection string, memID ulid.ULID, chunks []storage.Chunk) {
	if ftsIdx, ok := s.ftsIndexes[collection]; ok {
		// Index concatenated text for now (chunk-level FTS deferred).
		var fullText string
		for _, c := range chunks {
			fullText += c.Text + " "
		}
		ftsIdx.Add(memID, fullText)
	}

	if vecIdx, ok := s.vecIndexes[collection]; ok {
		// Use first chunk's vector as the document vector (MVP simplification).
		for _, c := range chunks {
			if c.Vector != nil {
				vecIdx.Add(memID, c.Vector)
				break
			}
		}
	}
}

func (s *Store) replayWAL() error {
	records, err := s.wal.Replay()
	if err != nil {
		return err
	}
	for _, rec := range records {
		id := ulid.ULID(rec.MemoryID)
		switch rec.Op {
		case storage.WALOpPut:
			var mem storage.Memory
			if err := json.Unmarshal(rec.Data, &mem); err != nil {
				continue
			}
			s.meta.PutMemory(&mem)
		case storage.WALOpDelete:
			s.meta.DeleteMemory(id, false)
		}
	}
	return s.wal.Truncate()
}

func toPublicMemory(m *storage.Memory) Memory {
	return Memory{
		ID:          m.ID.String(),
		Collection:  m.Collection,
		Content:     m.Content,
		ContentType: m.ContentType,
		Source:      m.Source,
		Type:        m.Type,
		Metadata:    m.Metadata,
		Importance:  m.Importance,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
		AccessedAt:  m.AccessedAt,
		AccessCount: m.AccessCount,
	}
}
