package ingest

import (
	"context"
	"crypto/rand"
	"strings"
	"testing"

	"github.com/oklog/ulid/v2"

	"github.com/xs-memory/xs-memory/internal/provider"
	"github.com/xs-memory/xs-memory/internal/storage"
)

func newID(t *testing.T) ulid.ULID {
	t.Helper()
	id, err := ulid.New(ulid.Now(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestIngestShortText(t *testing.T) {
	embedder := provider.NewMockEmbedder(64)
	p := NewPipeline(embedder, DefaultConfig())

	result, err := p.Ingest(context.Background(), IngestOpts{
		ID:         newID(t),
		Collection: "test",
		Content:    "Hello world, this is a short text.",
		Type:       storage.MemoryTypeSemantic,
	})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}

	if result.Memory.Collection != "test" {
		t.Errorf("collection = %q", result.Memory.Collection)
	}
	if len(result.Chunks) != 1 {
		t.Fatalf("chunks = %d, want 1 for short text", len(result.Chunks))
	}
	if len(result.Chunks[0].Vector) != 64 {
		t.Errorf("vector dim = %d, want 64", len(result.Chunks[0].Vector))
	}
	if result.Memory.BlobRef == "" {
		t.Error("BlobRef should be set")
	}
}

func TestIngestJapanese(t *testing.T) {
	embedder := provider.NewMockEmbedder(128)
	p := NewPipeline(embedder, DefaultConfig())

	result, err := p.Ingest(context.Background(), IngestOpts{
		ID:         newID(t),
		Collection: "ja-test",
		Content:    "東京タワーは東京都港区にある電波塔です。高さは333メートルです。",
		Type:       storage.MemoryTypeEpisodic,
	})
	if err != nil {
		t.Fatalf("Ingest Japanese: %v", err)
	}
	if len(result.Chunks) == 0 {
		t.Fatal("expected at least 1 chunk")
	}
	if len(result.Chunks[0].Vector) != 128 {
		t.Errorf("vector dim = %d", len(result.Chunks[0].Vector))
	}
}

func TestIngestLongTextChunking(t *testing.T) {
	embedder := provider.NewMockEmbedder(32)
	cfg := Config{ChunkTokens: 50, ChunkOverlap: 10}
	p := NewPipeline(embedder, cfg)

	// Build a long text.
	var sb strings.Builder
	for i := 0; i < 20; i++ {
		sb.WriteString("This is sentence number. ")
	}

	result, err := p.Ingest(context.Background(), IngestOpts{
		ID:         newID(t),
		Collection: "test",
		Content:    sb.String(),
		Type:       storage.MemoryTypeSemantic,
	})
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if len(result.Chunks) < 2 {
		t.Fatalf("expected multiple chunks for long text, got %d", len(result.Chunks))
	}
	for i, c := range result.Chunks {
		if len(c.Vector) != 32 {
			t.Errorf("chunk %d: vector dim = %d", i, len(c.Vector))
		}
		if c.Ord != i {
			t.Errorf("chunk %d: ord = %d", i, c.Ord)
		}
	}
}

func TestIngestWithoutEmbedder(t *testing.T) {
	// Degraded mode: no embedder configured. See N4.
	p := NewPipeline(nil, DefaultConfig())

	result, err := p.Ingest(context.Background(), IngestOpts{
		ID:         newID(t),
		Collection: "test",
		Content:    "No embeddings available",
		Type:       storage.MemoryTypeSemantic,
	})
	if err != nil {
		t.Fatalf("Ingest without embedder: %v", err)
	}
	if len(result.Chunks) == 0 {
		t.Fatal("should still produce chunks")
	}
	if result.Chunks[0].Vector != nil {
		t.Error("vector should be nil without embedder")
	}
}

func TestContentHash(t *testing.T) {
	p := NewPipeline(nil, DefaultConfig())
	content := "same content for dedup"

	r1, _ := p.Ingest(context.Background(), IngestOpts{
		ID: newID(t), Collection: "a", Content: content,
	})
	r2, _ := p.Ingest(context.Background(), IngestOpts{
		ID: newID(t), Collection: "b", Content: content,
	})

	if r1.Memory.BlobRef != r2.Memory.BlobRef {
		t.Error("same content should produce same BlobRef")
	}
}
