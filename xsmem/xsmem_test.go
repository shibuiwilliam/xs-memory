package xsmem_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/xs-memory/xs-memory/internal/provider"
	"github.com/xs-memory/xs-memory/xsmem"
)

func openTestStore(t *testing.T, opts ...xsmem.Option) *xsmem.Store {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.xsmem")

	s, err := xsmem.Open(path, opts...)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpenClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.xsmem")

	s, err := xsmem.Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	// Verify directory structure per design §6.1.
	for _, sub := range []string{"wal", "segments", "blobs"} {
		p := filepath.Join(path, sub)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected subdirectory %s: %v", sub, err)
		}
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Double close should error.
	if err := s.Close(); err == nil {
		t.Error("expected error on double Close")
	}
}

func TestRememberAndSearch(t *testing.T) {
	embedder := provider.NewMockEmbedder(64)
	s := openTestStore(t, xsmem.WithEmbedder(embedder))
	ctx := context.Background()

	// Create collection.
	err := s.CreateCollection("test", "en", "mock", 64)
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	// Store memories.
	id1, err := s.Remember(ctx, xsmem.RememberOpts{
		Collection: "test",
		Content:    "Go is a programming language designed at Google",
		Type:       xsmem.Semantic,
	})
	if err != nil {
		t.Fatalf("Remember: %v", err)
	}
	if id1 == "" {
		t.Fatal("expected non-empty ID")
	}

	_, err = s.Remember(ctx, xsmem.RememberOpts{
		Collection: "test",
		Content:    "Python is popular for data science",
		Type:       xsmem.Semantic,
	})
	if err != nil {
		t.Fatalf("Remember: %v", err)
	}

	// FTS search.
	results, err := s.Search(ctx, xsmem.SearchOpts{
		Collection: "test",
		Text:       "Go programming",
		Mode:       xsmem.FTS,
		TopK:       5,
	})
	if err != nil {
		t.Fatalf("Search FTS: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected FTS results")
	}

	// Hybrid search.
	results, err = s.Search(ctx, xsmem.SearchOpts{
		Collection: "test",
		Text:       "Go programming",
		Mode:       xsmem.Hybrid,
		TopK:       5,
	})
	if err != nil {
		t.Fatalf("Search Hybrid: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected Hybrid results")
	}
}

func TestRememberJapanese(t *testing.T) {
	embedder := provider.NewMockEmbedder(64)
	s := openTestStore(t, xsmem.WithEmbedder(embedder))
	ctx := context.Background()

	err := s.CreateCollection("ja", "ja", "mock", 64)
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	_, err = s.Remember(ctx, xsmem.RememberOpts{
		Collection: "ja",
		Content:    "東京タワーは東京都港区にある電波塔です",
		Type:       xsmem.Semantic,
	})
	if err != nil {
		t.Fatalf("Remember Japanese: %v", err)
	}

	_, err = s.Remember(ctx, xsmem.RememberOpts{
		Collection: "ja",
		Content:    "京都の金閣寺は美しい寺院です",
		Type:       xsmem.Episodic,
	})
	if err != nil {
		t.Fatalf("Remember: %v", err)
	}

	results, err := s.Search(ctx, xsmem.SearchOpts{
		Collection: "ja",
		Text:       "東京の電波塔",
		Mode:       xsmem.FTS,
		TopK:       5,
	})
	if err != nil {
		t.Fatalf("Search Japanese: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected Japanese FTS results")
	}
}

func TestGetUpdateForget(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	id, err := s.Remember(ctx, xsmem.RememberOpts{
		Content: "original content",
		Type:    xsmem.Semantic,
	})
	if err != nil {
		t.Fatalf("Remember: %v", err)
	}

	// Get.
	mem, err := s.Get(ctx, id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if mem.Content != "original content" {
		t.Errorf("content = %q", mem.Content)
	}

	// Update.
	newContent := "updated content"
	err = s.Update(ctx, id, xsmem.UpdateOpts{Content: &newContent})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	mem, _ = s.Get(ctx, id)
	if mem.Content != "updated content" {
		t.Errorf("after update: content = %q", mem.Content)
	}

	// Soft delete (default per N7).
	err = s.Forget(ctx, id, false)
	if err != nil {
		t.Fatalf("Forget: %v", err)
	}

	_, err = s.Get(ctx, id)
	if err == nil {
		t.Error("expected error after forget")
	}
}

func TestList(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	s.Remember(ctx, xsmem.RememberOpts{Content: "first memory"})
	s.Remember(ctx, xsmem.RememberOpts{Content: "second memory"})

	list, err := s.List(ctx, "default")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("List = %d, want 2", len(list))
	}
}

func TestStats(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	s.Remember(ctx, xsmem.RememberOpts{Content: "test"})

	stats := s.Stats()
	if stats.Memories != 1 {
		t.Errorf("memories = %d, want 1", stats.Memories)
	}
	if stats.BlockCacheStats.CapacityMB != 256 {
		t.Errorf("cache capacity = %f, want 256", stats.BlockCacheStats.CapacityMB)
	}
}

func TestSearchWithoutEmbedder(t *testing.T) {
	// Degraded mode: no embedder. N4.
	s := openTestStore(t)
	ctx := context.Background()

	s.Remember(ctx, xsmem.RememberOpts{Content: "test content for search"})

	// FTS should still work.
	results, err := s.Search(ctx, xsmem.SearchOpts{
		Text: "test content",
		Mode: xsmem.FTS,
		TopK: 5,
	})
	if err != nil {
		t.Fatalf("Search without embedder: %v", err)
	}
	if len(results) == 0 {
		t.Error("FTS should work without embedder")
	}
}
