// Package storage implements the persistence layer: WAL, segments, block cache,
// and bbolt metadata. See design §6.
package storage

import (
	"time"

	"github.com/oklog/ulid/v2"
)

// MemoryType distinguishes memory categories for scoring. See design §5.2.
type MemoryType string

const (
	MemoryTypeEpisodic   MemoryType = "episodic"
	MemoryTypeSemantic   MemoryType = "semantic"
	MemoryTypeProcedural MemoryType = "procedural"
)

// Memory is the core record. See design §5.1.
type Memory struct {
	ID          ulid.ULID
	Collection  string
	Content     string
	BlobRef     string // SHA-256 content-addressed hash, empty if inline
	ContentType string // text/plain, text/markdown, etc.
	Source      string // file://..., conversation:session-123, etc.
	Type        MemoryType
	Metadata    map[string]any
	Importance  float32
	CreatedAt   time.Time
	UpdatedAt   time.Time
	AccessedAt  time.Time
	AccessCount uint32
	Deleted     bool // tombstone flag, see design §6.4, N7
}

// Chunk is the search/vector unit, child of a Memory. See design §5.1.
type Chunk struct {
	ID       ulid.ULID
	MemoryID ulid.ULID
	Ord      int
	Text     string
	Vector   []float32
	Tokens   int
}

// EntityRef identifies a subject or object in a triple.
type EntityRef struct {
	ID   string // entity ID or memory ULID string
	Type string // "entity" or "memory"
}

// Triple is a knowledge graph edge. See design §5.1.
type Triple struct {
	S      EntityRef
	P      string // predicate
	O      EntityRef
	Weight float32
	Source ulid.ULID // originating Memory
}

// CollectionConfig holds immutable settings for a collection. See design §5.1, N6.
type CollectionConfig struct {
	Name           string `json:"name"`
	Analyzer       string `json:"analyzer"`        // "ja", "en", "bigram"
	EmbedderID     string `json:"embedder_id"`     // model identifier, pinned
	EmbedDimension int    `json:"embed_dimension"` // pinned, immutable
}

// Manifest is the store-level metadata file. See design §6.1.
type Manifest struct {
	Version     int                         `json:"version"`
	Collections map[string]CollectionConfig `json:"collections"`
}
