package search

import (
	"context"
	"crypto/rand"
	"math"
	"testing"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/xs-memory/xs-memory/internal/analyzer"
	"github.com/xs-memory/xs-memory/internal/index/fts"
	"github.com/xs-memory/xs-memory/internal/index/vector"
	"github.com/xs-memory/xs-memory/internal/provider"
)

func newID(t *testing.T) ulid.ULID {
	t.Helper()
	id, err := ulid.New(ulid.Now(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestFTSOnlySearch(t *testing.T) {
	a := analyzer.NewEnAnalyzer()
	idx := fts.NewIndex(a)

	doc1 := newID(t)
	doc2 := newID(t)
	idx.Add(doc1, "Go programming language concurrency")
	idx.Add(doc2, "Python scripting language automation")

	searcher := NewSearcher(idx, nil, nil)
	results, err := searcher.Search(context.Background(), Query{
		Text: "Go concurrency",
		Mode: ModeFTS,
		TopK: 5,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	if results[0].MemoryID != doc1 {
		t.Error("doc1 should rank first for 'Go concurrency'")
	}
}

func TestVectorOnlySearch(t *testing.T) {
	embedder := provider.NewMockEmbedder(64)
	vecIdx := vector.NewIndex(64, vector.Cosine, false)

	doc1 := newID(t)
	doc2 := newID(t)

	ctx := context.Background()
	v1, _ := embedder.Embed(ctx, []string{"Go programming"})
	v2, _ := embedder.Embed(ctx, []string{"cooking recipes"})
	vecIdx.Add(doc1, v1[0])
	vecIdx.Add(doc2, v2[0])

	searcher := NewSearcher(nil, vecIdx, embedder)
	results, err := searcher.Search(ctx, Query{
		Text: "Go programming",
		Mode: ModeVector,
		TopK: 5,
	})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	// doc1 should be most similar to "Go programming" query.
	if results[0].MemoryID != doc1 {
		t.Error("doc1 should rank first")
	}
}

func TestHybridRRFSearch(t *testing.T) {
	a := analyzer.NewEnAnalyzer()
	ftsIdx := fts.NewIndex(a)
	embedder := provider.NewMockEmbedder(64)
	vecIdx := vector.NewIndex(64, vector.Cosine, false)

	ctx := context.Background()

	doc1 := newID(t)
	doc2 := newID(t)
	doc3 := newID(t)

	// doc1: strong FTS match
	ftsIdx.Add(doc1, "Go programming language concurrent goroutines Go Go")
	// doc2: moderate match
	ftsIdx.Add(doc2, "Go language basics")
	// doc3: no FTS match
	ftsIdx.Add(doc3, "cooking delicious food")

	// Add vectors.
	v1, _ := embedder.Embed(ctx, []string{"Go programming"})
	v2, _ := embedder.Embed(ctx, []string{"Go language"})
	v3, _ := embedder.Embed(ctx, []string{"cooking food"})
	vecIdx.Add(doc1, v1[0])
	vecIdx.Add(doc2, v2[0])
	vecIdx.Add(doc3, v3[0])

	searcher := NewSearcher(ftsIdx, vecIdx, embedder)
	results, err := searcher.Search(ctx, Query{
		Text: "Go programming",
		Mode: ModeHybrid,
		TopK: 5,
	})
	if err != nil {
		t.Fatalf("Hybrid search: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
	// doc1 should rank first (strong in both FTS and vector).
	if results[0].MemoryID != doc1 {
		t.Errorf("doc1 should rank first in hybrid, got %s", results[0].MemoryID)
	}
}

func TestRRFDeterminism(t *testing.T) {
	// RRF fusion should be deterministic. See design §8.2.
	id1 := newID(t)
	id2 := newID(t)

	ftsResults := []Result{
		{MemoryID: id1, Score: 2.0},
		{MemoryID: id2, Score: 1.0},
	}
	vecResults := []Result{
		{MemoryID: id2, Score: 0.9},
		{MemoryID: id1, Score: 0.5},
	}

	r1 := rrfFuse(ftsResults, vecResults, 60, 10)
	r2 := rrfFuse(ftsResults, vecResults, 60, 10)

	if len(r1) != len(r2) {
		t.Fatalf("non-deterministic result count")
	}
	for i := range r1 {
		if r1[i].MemoryID != r2[i].MemoryID {
			t.Errorf("result %d: non-deterministic order", i)
		}
		if r1[i].Score != r2[i].Score {
			t.Errorf("result %d: non-deterministic score", i)
		}
	}
}

func TestRRFScoring(t *testing.T) {
	id1 := newID(t)
	id2 := newID(t)

	// id1 ranks #1 in both lists → should have highest RRF score.
	ftsResults := []Result{
		{MemoryID: id1, Score: 2.0},
		{MemoryID: id2, Score: 1.0},
	}
	vecResults := []Result{
		{MemoryID: id1, Score: 0.9},
		{MemoryID: id2, Score: 0.5},
	}

	results := rrfFuse(ftsResults, vecResults, 60, 10)
	if results[0].MemoryID != id1 {
		t.Error("id1 should rank first (rank 1 in both lists)")
	}
	// Expected: id1 score = 1/61 + 1/61 ≈ 0.0328
	expectedScore := 2.0 / 61.0
	if math.Abs(results[0].Score-expectedScore) > 0.001 {
		t.Errorf("score = %f, want %f", results[0].Score, expectedScore)
	}
}

func TestRecencyScore(t *testing.T) {
	now := time.Now()
	halfLife := 72 * time.Hour

	// Just accessed → score ≈ 1.0
	s := RecencyScore(now, now, halfLife)
	if math.Abs(s-1.0) > 0.01 {
		t.Errorf("recency(now) = %f, want ~1.0", s)
	}

	// Half-life ago → score ≈ 0.5
	s = RecencyScore(now, now.Add(-halfLife), halfLife)
	if math.Abs(s-0.5) > 0.01 {
		t.Errorf("recency(half-life) = %f, want ~0.5", s)
	}

	// Two half-lives → score ≈ 0.25
	s = RecencyScore(now, now.Add(-2*halfLife), halfLife)
	if math.Abs(s-0.25) > 0.01 {
		t.Errorf("recency(2x half-life) = %f, want ~0.25", s)
	}
}
