package xsmem_test

import (
	"context"
	"testing"

	"github.com/xs-memory/xs-memory/internal/metrics"
	"github.com/xs-memory/xs-memory/xsmem"
)

func TestMetricsDisabledByDefault(t *testing.T) {
	// Default config has metrics disabled → Store holds NopRecorder.
	// See addendum3 M1.
	s := openTestStore(t)
	if s.MetricsEnabled() {
		t.Error("metrics should be disabled by default")
	}

	// Searches should work without error; snapshot should be empty.
	ctx := context.Background()
	s.Remember(ctx, xsmem.RememberOpts{Content: "test content"})
	s.Search(ctx, xsmem.SearchOpts{Text: "test", Mode: xsmem.FTS, TopK: 5})

	snap := s.MetricsSnapshot("")
	if snap.TotalSearches != 0 {
		t.Errorf("disabled metrics should report 0 searches, got %d", snap.TotalSearches)
	}
}

func TestMetricsEnabledRecordsCounts(t *testing.T) {
	// Enable metrics and verify search recording. See addendum3 §1.1, §1.3.
	cfg := metrics.DefaultConfig()
	cfg.Enabled = true
	s := openTestStore(t, xsmem.WithMetrics(cfg))

	if !s.MetricsEnabled() {
		t.Fatal("metrics should be enabled")
	}

	ctx := context.Background()
	s.Remember(ctx, xsmem.RememberOpts{Content: "Go is a programming language"})
	s.Remember(ctx, xsmem.RememberOpts{Content: "Python is popular"})

	// FTS search.
	s.Search(ctx, xsmem.SearchOpts{Text: "Go programming", Mode: xsmem.FTS, TopK: 10})
	// Hybrid search.
	s.Search(ctx, xsmem.SearchOpts{Text: "programming", Mode: xsmem.Hybrid, TopK: 10})
	s.Search(ctx, xsmem.SearchOpts{Text: "Python", Mode: xsmem.Hybrid, TopK: 10})

	snap := s.MetricsSnapshot("")
	if snap.TotalSearches != 3 {
		t.Errorf("total = %d, want 3", snap.TotalSearches)
	}

	// Mode distribution.
	snap.ComputeModeDistribution()
	if snap.ModeDistribution == nil {
		t.Fatal("ModeDistribution should be computed")
	}
}

func TestMetricsHitRate(t *testing.T) {
	cfg := metrics.DefaultConfig()
	cfg.Enabled = true
	s := openTestStore(t, xsmem.WithMetrics(cfg))

	ctx := context.Background()
	s.Remember(ctx, xsmem.RememberOpts{Content: "only one memory"})

	// Request 10 but only 1 exists → underfilled.
	s.Search(ctx, xsmem.SearchOpts{Text: "one", Mode: xsmem.FTS, TopK: 10})

	snap := s.MetricsSnapshot("")
	ftsUnder := snap.Underfilled[metrics.ModeFTS]
	if ftsUnder != 1 {
		t.Errorf("underfilled = %d, want 1", ftsUnder)
	}
}

func TestMetricsReset(t *testing.T) {
	cfg := metrics.DefaultConfig()
	cfg.Enabled = true
	s := openTestStore(t, xsmem.WithMetrics(cfg))

	ctx := context.Background()
	s.Remember(ctx, xsmem.RememberOpts{Content: "some content"})
	s.Search(ctx, xsmem.SearchOpts{Text: "content", Mode: xsmem.FTS, TopK: 5})

	s.MetricsReset("")
	snap := s.MetricsSnapshot("")
	if snap.TotalSearches != 0 {
		t.Errorf("after reset total = %d, want 0", snap.TotalSearches)
	}
}
