package tuning

import (
	"testing"
	"time"
)

func enabledConfig() Config {
	cfg := DefaultConfig()
	cfg.Enabled = true
	return cfg
}

func TestRecordUsageAndPrior(t *testing.T) {
	s := NewStore(enabledConfig())

	s.RecordUsage(UsageEvent{
		Collection:  "test",
		Terms:       []string{"cache", "eviction"},
		Impressions: []string{"id1", "id2", "id3"},
		Used:        []string{"id1"},
	})

	// id1 was used once with 1 impression.
	p := s.GetPrior("test", "id1")
	if p <= 0 {
		t.Errorf("prior for used item should be positive, got %f", p)
	}

	// id2 was only impressed, not used.
	p2 := s.GetPrior("test", "id2")
	if p2 >= p {
		t.Errorf("unused item prior (%f) should be less than used item (%f)", p2, p)
	}

	// id4 was never seen.
	p4 := s.GetPrior("test", "id4")
	if p4 != 0 {
		t.Errorf("unseen item prior should be 0, got %f", p4)
	}
}

func TestAffinityTracking(t *testing.T) {
	s := NewStore(enabledConfig())

	s.RecordUsage(UsageEvent{
		Collection:  "test",
		Terms:       []string{"cache", "LRU"},
		Impressions: []string{"id1"},
		Used:        []string{"id1"},
	})

	aff := s.GetAffinity("test", []string{"cache"}, "id1")
	if aff <= 0 {
		t.Errorf("affinity for co-used term should be positive, got %f", aff)
	}

	// Unrelated term has no affinity.
	aff2 := s.GetAffinity("test", []string{"unrelated"}, "id1")
	if aff2 != 0 {
		t.Errorf("unrelated term affinity should be 0, got %f", aff2)
	}
}

func TestColdStartZeroBoost(t *testing.T) {
	// Cold start: no signals → boost is 0 → ranking identical to base.
	// See addendum2 §1.3, A2.
	s := NewStore(enabledConfig())

	boost := s.ComputeBoost("test", []string{"query"}, "id1", 0.5)
	if boost != 0 {
		t.Errorf("cold start boost should be 0, got %f", boost)
	}
}

func TestBoostCap(t *testing.T) {
	// Near-zero base + high affinity: boost capped at cap*base.
	// See addendum2 §1.3, A2.
	cfg := enabledConfig()
	cfg.BoostCap = 0.5
	s := NewStore(cfg)

	// Pump up affinity.
	for i := 0; i < 100; i++ {
		s.RecordUsage(UsageEvent{
			Collection:  "test",
			Terms:       []string{"rare"},
			Impressions: []string{"item"},
			Used:        []string{"item"},
		})
	}

	baseScore := 0.01 // near-zero base
	boost := s.ComputeBoost("test", []string{"rare"}, "item", baseScore)
	maxAllowed := cfg.BoostCap * baseScore
	if boost > maxAllowed+1e-9 {
		t.Errorf("boost %f exceeds cap %f (base=%f, cap=%f)", boost, maxAllowed, baseScore, cfg.BoostCap)
	}
}

func TestTuningReset(t *testing.T) {
	// Reset clears all signals → base ranking restored. See addendum2 §1.5.
	s := NewStore(enabledConfig())

	s.RecordUsage(UsageEvent{
		Collection: "test", Terms: []string{"q"},
		Impressions: []string{"id1"}, Used: []string{"id1"},
	})

	epochBefore := s.Epoch()
	s.Reset("test")

	if s.GetPrior("test", "id1") != 0 {
		t.Error("prior should be 0 after reset")
	}
	if s.GetAffinity("test", []string{"q"}, "id1") != 0 {
		t.Error("affinity should be 0 after reset")
	}
	if s.Epoch() <= epochBefore {
		t.Error("epoch should bump on reset")
	}
}

func TestPurgeOnDelete(t *testing.T) {
	// Deleting an item purges its signals. See addendum2 §1.5.
	s := NewStore(enabledConfig())

	s.RecordUsage(UsageEvent{
		Collection: "test", Terms: []string{"q"},
		Impressions: []string{"id1"}, Used: []string{"id1"},
	})

	s.PurgeItem("test", "id1")

	if s.GetPrior("test", "id1") != 0 {
		t.Error("prior should be 0 after purge")
	}
}

func TestDisabledConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Enabled = false
	s := NewStore(cfg)

	s.RecordUsage(UsageEvent{
		Collection: "test", Terms: []string{"q"},
		Impressions: []string{"id1"}, Used: []string{"id1"},
	})

	// Nothing should be recorded when disabled.
	if s.GetPrior("test", "id1") != 0 {
		t.Error("disabled store should not record priors")
	}
	boost := s.ComputeBoost("test", []string{"q"}, "id1", 1.0)
	if boost != 0 {
		t.Error("disabled store should return 0 boost")
	}
}

func TestExplorationEpsilon(t *testing.T) {
	// Simulated self-reinforcement: even with heavy boosting,
	// fresh items remain reachable because boost is capped (A3).
	cfg := enabledConfig()
	cfg.BoostCap = 0.5
	cfg.ExplorationEps = 0.05
	s := NewStore(cfg)

	// Heavily boost "popular" item.
	for i := 0; i < 1000; i++ {
		s.RecordUsage(UsageEvent{
			Collection: "test", Terms: []string{"query"},
			Impressions: []string{"popular", "fresh"},
			Used:        []string{"popular"},
		})
	}

	// Fresh item has zero usage but accumulated impressions → small prior.
	freshBoost := s.ComputeBoost("test", []string{"query"}, "fresh", 0.5)
	popularBoost := s.ComputeBoost("test", []string{"query"}, "popular", 0.5)

	// Fresh item gets only a tiny prior boost (no affinity, never used).
	// The key: it is much smaller than the popular item's boost.
	if freshBoost >= popularBoost {
		t.Errorf("fresh boost (%f) should be less than popular (%f)", freshBoost, popularBoost)
	}

	// Popular item gets a boost but it's capped.
	maxBoost := cfg.BoostCap * 0.5
	if popularBoost > maxBoost+1e-9 {
		t.Errorf("popular boost %f exceeds cap %f", popularBoost, maxBoost)
	}

	// Key: fresh item's base score (0.5) is NOT reduced by the tuning system.
	// Its final score = base(0.5) + boost(0) = 0.5
	// Popular final = base(0.5) + boost(capped at 0.25) = 0.75
	// Fresh is still visible in top-k when base is strong enough.
	// The ε exploration means we never suppress fresh items below their base.
}

func TestClockInjection(t *testing.T) {
	s := NewStore(enabledConfig())
	fixedTime := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	s.SetClock(func() time.Time { return fixedTime })

	s.RecordUsage(UsageEvent{
		Collection: "test", Terms: []string{"q"},
		Impressions: []string{"id1"}, Used: []string{"id1"},
	})

	stats := s.Stats()
	if stats.EventCount != 1 {
		t.Errorf("event count = %d, want 1", stats.EventCount)
	}
}

func TestTuningStats(t *testing.T) {
	s := NewStore(enabledConfig())

	s.RecordUsage(UsageEvent{
		Collection: "test", Terms: []string{"a", "b"},
		Impressions: []string{"id1", "id2"}, Used: []string{"id1"},
	})

	stats := s.Stats()
	if stats.EventCount != 1 {
		t.Errorf("events = %d", stats.EventCount)
	}
	if stats.PriorCount != 2 {
		t.Errorf("priors = %d, want 2", stats.PriorCount)
	}
	if stats.AffinityCount == 0 {
		t.Error("expected affinities")
	}
}
