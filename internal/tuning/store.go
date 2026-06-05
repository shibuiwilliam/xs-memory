// Package tuning implements usage-driven adaptive ranking.
// Stores usage signals in a separate namespace, never mutating immutable segments.
// See addendum2 §1.
package tuning

import (
	"sync"
	"sync/atomic"
	"time"
)

// UsageEvent records a single search usage event. See addendum2 §1.1.
type UsageEvent struct {
	QueryID     string
	Collection  string
	Terms       []string // analyzer tokens (optionally hashed)
	Impressions []string // chunk/memory IDs shown
	Used        []string // chunk/memory IDs actually consumed
	At          time.Time
}

// ItemPrior tracks query-independent usefulness of an item. See addendum2 §1.2a.
type ItemPrior struct {
	Used    float64
	Impr    float64
	Decayed time.Time
}

// Config controls the tuning subsystem. See addendum2 §4.
type Config struct {
	Enabled          bool
	HashTerms        bool
	HalfLifePrior    time.Duration
	HalfLifeAffinity time.Duration
	WPrior           float64
	WAffinity        float64
	BoostCap         float64 // max adaptive contribution as fraction of base
	ExplorationEps   float64 // ε for exploration
	MaxItemsPerTerm  int
	MaxTermsPerItem  int
	FusionWeightBand float64 // ± clamp around defaults
}

// DefaultConfig returns default tuning configuration. See addendum2 §4.
func DefaultConfig() Config {
	return Config{
		Enabled:          false, // off by default
		HalfLifePrior:    30 * 24 * time.Hour,
		HalfLifeAffinity: 21 * 24 * time.Hour,
		WPrior:           0.15,
		WAffinity:        0.20,
		BoostCap:         0.5,
		ExplorationEps:   0.05,
		MaxItemsPerTerm:  32,
		MaxTermsPerItem:  32,
		FusionWeightBand: 0.30,
	}
}

// Store holds usage signals and learned aggregates. Lives in a separate
// namespace, independent of immutable segments. See addendum2 §1.5, A1, A5.
type Store struct {
	mu    sync.RWMutex
	cfg   Config
	epoch atomic.Uint64 // bumped on reset; used in cache key

	// Per-collection data.
	priors     map[string]map[string]*ItemPrior         // collection → itemID → prior
	affinities map[string]map[string]map[string]float64 // collection → term → itemID → weight
	events     []UsageEvent

	// Clock for deterministic testing. See addendum2 §6.
	now func() time.Time
}

// NewStore creates a tuning store. See addendum2 §1.5.
func NewStore(cfg Config) *Store {
	return &Store{
		cfg:        cfg,
		priors:     make(map[string]map[string]*ItemPrior),
		affinities: make(map[string]map[string]map[string]float64),
		now:        time.Now,
	}
}

// SetClock injects a clock for testing. See addendum2 §6.
func (s *Store) SetClock(fn func() time.Time) {
	s.now = fn
}

// Epoch returns the current tuning epoch. Bumped on reset.
func (s *Store) Epoch() uint64 {
	return s.epoch.Load()
}

// RecordUsage records a usage event. See addendum2 §1.1.
func (s *Store) RecordUsage(event UsageEvent) {
	if !s.cfg.Enabled {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if event.At.IsZero() {
		event.At = s.now()
	}
	s.events = append(s.events, event)

	col := event.Collection
	if col == "" {
		col = "default"
	}

	// Update item priors. See addendum2 §1.2a.
	if s.priors[col] == nil {
		s.priors[col] = make(map[string]*ItemPrior)
	}
	for _, id := range event.Impressions {
		p, ok := s.priors[col][id]
		if !ok {
			p = &ItemPrior{}
			s.priors[col][id] = p
		}
		p.Impr++
		p.Decayed = event.At
	}
	for _, id := range event.Used {
		p, ok := s.priors[col][id]
		if !ok {
			p = &ItemPrior{Decayed: event.At}
			s.priors[col][id] = p
		}
		p.Used++
	}

	// Update affinities. See addendum2 §1.2b.
	if s.affinities[col] == nil {
		s.affinities[col] = make(map[string]map[string]float64)
	}
	for _, term := range event.Terms {
		if s.affinities[col][term] == nil {
			s.affinities[col][term] = make(map[string]float64)
		}
		for _, id := range event.Used {
			s.affinities[col][term][id] += 1.0
			// Bound: keep top-N items per term. See addendum2 §1.2b.
			if len(s.affinities[col][term]) > s.cfg.MaxItemsPerTerm {
				s.pruneAffinityTerm(col, term)
			}
		}
	}
}

// GetPrior returns the smoothed usage prior for an item. See addendum2 §1.2a.
func (s *Store) GetPrior(collection, itemID string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	priors := s.priors[collection]
	if priors == nil {
		return 0
	}
	p, ok := priors[itemID]
	if !ok {
		return 0
	}

	// Smoothed rate: u(d) = (used + α) / (impr + α + β)
	// α=1, β=2 for Wilson-style smoothing.
	const alpha, beta = 1.0, 2.0
	return (p.Used + alpha) / (p.Impr + alpha + beta)
}

// GetAffinity returns the query-term → item affinity score. See addendum2 §1.2b.
func (s *Store) GetAffinity(collection string, terms []string, itemID string) float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	aff := s.affinities[collection]
	if aff == nil {
		return 0
	}

	var total float64
	for _, term := range terms {
		if items, ok := aff[term]; ok {
			total += items[itemID]
		}
	}
	return total
}

// ComputeBoost computes the additive adaptive boost for an item.
// Capped relative to base score. See addendum2 §1.3.
func (s *Store) ComputeBoost(collection string, terms []string, itemID string, baseScore float64) float64 {
	if !s.cfg.Enabled {
		return 0 // cold start = base behavior (A2)
	}

	prior := s.GetPrior(collection, itemID)
	affinity := s.GetAffinity(collection, terms, itemID)

	boost := s.cfg.WPrior*prior + s.cfg.WAffinity*affinity

	// Cap: boost ≤ cap * base. See addendum2 §1.3.
	maxBoost := s.cfg.BoostCap * baseScore
	if boost > maxBoost {
		boost = maxBoost
	}

	return boost
}

// PurgeItem removes all signals for an item. Called on forget/merge.
// See addendum2 §1.5.
func (s *Store) PurgeItem(collection, itemID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if priors := s.priors[collection]; priors != nil {
		delete(priors, itemID)
	}

	if aff := s.affinities[collection]; aff != nil {
		for term := range aff {
			delete(aff[term], itemID)
		}
	}
}

// Reset clears all learned signals → instant return to base behavior.
// Bumps the epoch to invalidate cache entries. See addendum2 §1.5.
func (s *Store) Reset(collection string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if collection == "" {
		// Reset all.
		s.priors = make(map[string]map[string]*ItemPrior)
		s.affinities = make(map[string]map[string]map[string]float64)
		s.events = nil
	} else {
		delete(s.priors, collection)
		delete(s.affinities, collection)
	}

	s.epoch.Add(1)
}

// Stats returns tuning store statistics.
type TuningStats struct {
	Epoch         uint64 `json:"epoch"`
	EventCount    int    `json:"event_count"`
	PriorCount    int    `json:"prior_count"`    // total items with priors
	AffinityCount int    `json:"affinity_count"` // total term→item pairs
}

func (s *Store) Stats() TuningStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var priorCount, affinityCount int
	for _, priors := range s.priors {
		priorCount += len(priors)
	}
	for _, terms := range s.affinities {
		for _, items := range terms {
			affinityCount += len(items)
		}
	}

	return TuningStats{
		Epoch:         s.epoch.Load(),
		EventCount:    len(s.events),
		PriorCount:    priorCount,
		AffinityCount: affinityCount,
	}
}

func (s *Store) pruneAffinityTerm(collection, term string) {
	items := s.affinities[collection][term]
	if len(items) <= s.cfg.MaxItemsPerTerm {
		return
	}
	// Find minimum and remove it.
	var minKey string
	minVal := float64(1<<63 - 1)
	for k, v := range items {
		if v < minVal {
			minVal = v
			minKey = k
		}
	}
	delete(items, minKey)
}
