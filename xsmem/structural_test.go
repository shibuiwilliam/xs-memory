package xsmem_test

import (
	"context"
	"testing"

	"github.com/xs-memory/xs-memory/internal/provider"
	"github.com/xs-memory/xs-memory/xsmem"
)

func TestStructuralStatsOracle(t *testing.T) {
	// Seed a store with known data and verify structural stats match.
	// See addendum3 §1.6.
	s := openTestStore(t)
	ctx := context.Background()

	s.CreateCollection("en", "en", "", 0)
	s.Remember(ctx, xsmem.RememberOpts{Collection: "en", Content: "Go is fast"})
	s.Remember(ctx, xsmem.RememberOpts{Collection: "en", Content: "Rust is safe"})

	stats := s.StructuralStats()
	if stats.Memories != 2 {
		t.Errorf("memories = %d, want 2", stats.Memories)
	}
	if stats.FTSTermCount == 0 {
		t.Error("FTS term count should be > 0 after indexing")
	}
	if stats.FTSDocCount != 2 {
		t.Errorf("FTS doc count = %d, want 2", stats.FTSDocCount)
	}
	if stats.Collections < 1 {
		t.Errorf("collections = %d, want ≥ 1", stats.Collections)
	}
}

func TestStructuralStatsCachedByGeneration(t *testing.T) {
	// Cached value reused between writes, recomputed after a write.
	// See addendum3 M5.
	s := openTestStore(t)
	ctx := context.Background()

	s.Remember(ctx, xsmem.RememberOpts{Content: "initial content"})

	stats1 := s.StructuralStats()
	stats2 := s.StructuralStats()

	// Same result between writes (cached).
	if stats1.FTSTermCount != stats2.FTSTermCount {
		t.Error("structural stats should be identical between writes (cached)")
	}

	// Add more data → generation bumps → cache should recompute.
	s.Remember(ctx, xsmem.RememberOpts{Content: "additional novel words"})

	stats3 := s.StructuralStats()
	if stats3.Memories <= stats1.Memories {
		t.Errorf("memories should increase after write: before=%d, after=%d",
			stats1.Memories, stats3.Memories)
	}
	if stats3.FTSTermCount <= stats1.FTSTermCount {
		t.Errorf("FTS terms should increase after indexing new content: before=%d, after=%d",
			stats1.FTSTermCount, stats3.FTSTermCount)
	}
}

func TestStructuralStatsVectorIndex(t *testing.T) {
	// With an embedder configured, verify vector stats.
	embedder := provider.NewMockEmbedder(64)
	s := openTestStore(t, xsmem.WithEmbedder(embedder))
	ctx := context.Background()

	s.CreateCollection("vec", "en", "mock", 64)
	s.Remember(ctx, xsmem.RememberOpts{Collection: "vec", Content: "vector content"})

	stats := s.StructuralStats()
	if stats.VectorCount != 1 {
		t.Errorf("vector count = %d, want 1", stats.VectorCount)
	}
	if stats.VectorDim != 64 {
		t.Errorf("vector dim = %d, want 64", stats.VectorDim)
	}
}

func TestStructuralStatsGraphEdges(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	s.Link(ctx, xsmem.Triple{Subject: "A", Predicate: "knows", Object: "B", Weight: 1.0})
	s.Link(ctx, xsmem.Triple{Subject: "B", Predicate: "knows", Object: "C", Weight: 1.0})

	stats := s.StructuralStats()
	if stats.GraphEdgeCount != 2 {
		t.Errorf("graph edges = %d, want 2", stats.GraphEdgeCount)
	}
}
