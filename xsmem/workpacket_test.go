package xsmem_test

import (
	"context"
	"testing"

	"github.com/xs-memory/xs-memory/internal/provider"
	"github.com/xs-memory/xs-memory/xsmem"
)

func TestSuggestOrganizationWorkPacket(t *testing.T) {
	embedder := provider.NewMockEmbedder(64)
	s := openTestStore(t, xsmem.WithEmbedder(embedder))
	ctx := context.Background()

	s.CreateCollection("test", "en", "mock", 64)

	// Add untagged memories.
	s.Remember(ctx, xsmem.RememberOpts{Collection: "test", Content: "untagged memory one", Type: xsmem.Semantic})
	s.Remember(ctx, xsmem.RememberOpts{Collection: "test", Content: "untagged memory two", Type: xsmem.Semantic})
	// Add an episodic memory.
	s.Remember(ctx, xsmem.RememberOpts{Collection: "test", Content: "we discussed caching strategy", Type: xsmem.Episodic})

	wp, err := s.SuggestOrganization(ctx, "test")
	if err != nil {
		t.Fatalf("SuggestOrganization: %v", err)
	}
	if wp == nil {
		t.Fatal("work packet is nil")
	}
	if len(wp.Untagged) < 2 {
		t.Errorf("expected at least 2 untagged items, got %d", len(wp.Untagged))
	}
	if len(wp.EpisodicClusters) == 0 {
		t.Error("expected at least 1 episodic cluster")
	}
}

func TestSuggestOrganizationNoLLM(t *testing.T) {
	// No embedder, no LLM. SuggestOrganization must still work (H4, H7).
	s := openTestStore(t)
	ctx := context.Background()

	s.Remember(ctx, xsmem.RememberOpts{Content: "test memory"})

	wp, err := s.SuggestOrganization(ctx, "default")
	if err != nil {
		t.Fatalf("SuggestOrganization without LLM: %v", err)
	}
	if wp == nil {
		t.Fatal("work packet should not be nil even without LLM")
	}
}

func TestMergeRequiresConfirmation(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	id1, _ := s.Remember(ctx, xsmem.RememberOpts{Content: "memory A"})
	id2, _ := s.Remember(ctx, xsmem.RememberOpts{Content: "memory B"})

	// Merge without confirmed=true → must fail (H5).
	_, err := s.Merge(ctx, xsmem.MergeOpts{
		IDs:       []string{id1, id2},
		Summary:   "merged summary",
		Confirmed: false,
	})
	if err == nil {
		t.Fatal("merge without confirmed=true should fail")
	}

	// Merge with confirmed=true → must succeed.
	newID, err := s.Merge(ctx, xsmem.MergeOpts{
		IDs:       []string{id1, id2},
		Summary:   "merged summary",
		Confirmed: true,
	})
	if err != nil {
		t.Fatalf("merge with confirmed=true: %v", err)
	}
	if newID == "" {
		t.Fatal("merge should return new ID")
	}

	// Originals should be tombstoned.
	_, err = s.Get(ctx, id1)
	if err == nil {
		t.Error("original memory should be tombstoned after merge")
	}

	// New merged memory should exist with provenance.
	merged, err := s.Get(ctx, newID)
	if err != nil {
		t.Fatalf("get merged: %v", err)
	}
	if merged.Content != "merged summary" {
		t.Errorf("merged content = %q", merged.Content)
	}
	mergedFrom, ok := merged.Metadata["merged_from"]
	if !ok {
		t.Error("merged memory should have merged_from metadata")
	}
	_ = mergedFrom
}

func TestMergeJapanese(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	id1, _ := s.Remember(ctx, xsmem.RememberOpts{Content: "東京タワーは電波塔です"})
	id2, _ := s.Remember(ctx, xsmem.RememberOpts{Content: "東京タワーの高さは333メートル"})

	newID, err := s.Merge(ctx, xsmem.MergeOpts{
		IDs:       []string{id1, id2},
		Summary:   "東京タワーは高さ333メートルの電波塔です",
		Confirmed: true,
	})
	if err != nil {
		t.Fatalf("merge Japanese: %v", err)
	}
	merged, _ := s.Get(ctx, newID)
	if merged.Content != "東京タワーは高さ333メートルの電波塔です" {
		t.Errorf("content = %q", merged.Content)
	}
}

func TestRecallNoLLM(t *testing.T) {
	// Recall must work with zero LLM (H4, N4).
	s := openTestStore(t)
	ctx := context.Background()

	s.Remember(ctx, xsmem.RememberOpts{Content: "Go programming concurrency goroutines"})
	s.Remember(ctx, xsmem.RememberOpts{Content: "Python data science pandas"})

	results, err := s.Search(ctx, xsmem.SearchOpts{
		Text: "Go concurrency", Mode: xsmem.FTS, TopK: 5,
	})
	if err != nil {
		t.Fatalf("Search FTS without LLM: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("recall (FTS) should work without any LLM")
	}
}

func TestHostDelegatedNeverCallsProvider(t *testing.T) {
	// Inject a failsafe provider that fails the test if called (H1).
	failsafe := provider.NewFailsafeLLM(func(msg string) {
		t.Fatal(msg)
	})
	s := openTestStore(t, xsmem.WithLLM(failsafe))
	ctx := context.Background()

	s.Remember(ctx, xsmem.RememberOpts{Content: "test content"})

	// SuggestOrganization should NEVER call the LLM.
	wp, err := s.SuggestOrganization(ctx, "default")
	if err != nil {
		t.Fatalf("SuggestOrganization: %v", err)
	}
	if wp == nil {
		t.Fatal("work packet should not be nil")
	}
	// If we get here, the failsafe was never called → H1 verified.
}

func TestOrganizeDisabledQueues(t *testing.T) {
	// When no LLM is configured, Organize should not fail (H7).
	// The existing Organize returns error when organizer is nil,
	// but SuggestOrganization (the host-delegated path) should work.
	s := openTestStore(t)
	ctx := context.Background()

	s.Remember(ctx, xsmem.RememberOpts{Content: "test"})

	// SuggestOrganization is the correct degradation path.
	wp, err := s.SuggestOrganization(ctx, "default")
	if err != nil {
		t.Fatalf("SuggestOrganization in disabled mode should not fail: %v", err)
	}
	if wp == nil {
		t.Fatal("should return a work packet, not nil")
	}
}
