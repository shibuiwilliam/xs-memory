// Package metrics implements an optional, privacy-by-design, local-only metrics
// subsystem for xs-memory. When disabled, the search path holds a no-op recorder:
// zero allocations, no measurable latency delta. See addendum3 §0, M1.
package metrics

import (
	"sync"
	"sync/atomic"
	"time"
)

// Mode identifies a search mode for metrics keying. See addendum3 §1.1.
type Mode string

const (
	ModeFTS    Mode = "fts"
	ModeVector Mode = "vector"
	ModeHybrid Mode = "hybrid"
	ModeGrep   Mode = "grep"
)

// AllModes is the fixed set of modes for bounded cardinality. See addendum3 M7.
var AllModes = []Mode{ModeFTS, ModeVector, ModeHybrid, ModeGrep}

// Config controls the metrics subsystem. See addendum3 §4.
type Config struct {
	Enabled bool // off by default (M1)

	// Search sub-metrics. See addendum3 §1.
	SearchCount    bool // per (collection, mode) counters
	HitRate        bool // returned vs requested topK
	ModeDistrib    bool // derived from count (M4)
	LatencyEnabled bool // coarse buckets, off by default (M3)

	// Keyword metrics. See addendum3 §1.2, M2.
	KeywordsEnabled bool
	KeywordsHashed  bool // default true; shares tuning salt
	KeywordsRaw     bool // explicit local-only opt-in
	KeywordsTopK    int  // bounded (M7)

	// Index-level structural stats. See addendum3 §1.6, M5.
	IndexStats bool
}

// DefaultConfig returns default metrics configuration. See addendum3 §4.
func DefaultConfig() Config {
	return Config{
		Enabled:         false, // off by default (M1)
		SearchCount:     true,
		HitRate:         true,
		ModeDistrib:     true,
		LatencyEnabled:  false, // no timing by default (M3)
		KeywordsEnabled: true,
		KeywordsHashed:  true,
		KeywordsRaw:     false, // privacy-by-design (M2)
		KeywordsTopK:    200,   // bounded (M7)
		IndexStats:      true,
	}
}

// Recorder is the metrics recording interface. When metrics are disabled,
// the Store holds a NopRecorder — zero allocations, zero overhead on the
// search path. See addendum3 §1, M1.
type Recorder interface {
	// RecordSearch records a completed search. See addendum3 §1.1, §1.3, §1.4.
	RecordSearch(collection string, mode Mode, returned, requested int, latency time.Duration)

	// RecordTerms records hashed query tokens. See addendum3 §1.2, M2.
	RecordTerms(collection string, tokens []string)

	// Snapshot returns a point-in-time snapshot of all metrics. See addendum3 §3.
	Snapshot(collection string) *Snapshot

	// Reset clears all aggregates. See addendum3 §2.
	Reset(collection string)

	// IsEnabled returns true if the recorder is active.
	IsEnabled() bool
}

// --- NopRecorder: the disabled path. Zero cost. See addendum3 M1. ---

// NopRecorder is used when metrics.enabled = false. Every method is a no-op.
// The search path incurs zero allocations and no measurable overhead.
type NopRecorder struct{}

func (NopRecorder) RecordSearch(string, Mode, int, int, time.Duration) {}
func (NopRecorder) RecordTerms(string, []string)                       {}
func (NopRecorder) Snapshot(string) *Snapshot                          { return &Snapshot{} }
func (NopRecorder) Reset(string)                                       {}
func (NopRecorder) IsEnabled() bool                                    { return false }

// --- LiveRecorder: the enabled path. Bounded, sharded, atomic. ---

// LiveRecorder captures metrics when enabled. Uses sharded atomic counters
// and fixed-bucket histograms for bounded memory. See addendum3 §1, §2, M7.
type LiveRecorder struct {
	cfg Config
	mu  sync.RWMutex

	// Search counts per (collection, mode). See addendum3 §1.1.
	searchCounts map[string]*[4]atomic.Uint64 // indexed by mode ordinal

	// Hit rate: returned and requested totals per (collection, mode). See addendum3 §1.3.
	returned    map[string]*[4]atomic.Uint64
	requested   map[string]*[4]atomic.Uint64
	underfilled map[string]*[4]atomic.Uint64 // returned < requested

	// Latency histograms per (collection, mode). See addendum3 §1.4, M3.
	latencyHist map[string]*[4]LatencyHistogram

	// Keyword top-K. See addendum3 §1.2.
	topK map[string]*TopKTerms
}

// NewLiveRecorder creates an active recorder. See addendum3 §2.
func NewLiveRecorder(cfg Config) *LiveRecorder {
	return &LiveRecorder{
		cfg:          cfg,
		searchCounts: make(map[string]*[4]atomic.Uint64),
		returned:     make(map[string]*[4]atomic.Uint64),
		requested:    make(map[string]*[4]atomic.Uint64),
		underfilled:  make(map[string]*[4]atomic.Uint64),
		latencyHist:  make(map[string]*[4]LatencyHistogram),
		topK:         make(map[string]*TopKTerms),
	}
}

// modeOrd maps a Mode to its ordinal index [0..3] for array indexing.
func modeOrd(m Mode) int {
	switch m {
	case ModeFTS:
		return 0
	case ModeVector:
		return 1
	case ModeHybrid:
		return 2
	case ModeGrep:
		return 3
	default:
		return 2 // default to hybrid
	}
}

// getOrCreate retrieves or initializes the per-collection arrays.
// Uses a read-first pattern with promotion to write lock on miss.
func (r *LiveRecorder) getOrCreate(collection string) (
	counts *[4]atomic.Uint64,
	ret *[4]atomic.Uint64,
	req *[4]atomic.Uint64,
	under *[4]atomic.Uint64,
	hist *[4]LatencyHistogram,
) {
	r.mu.RLock()
	counts = r.searchCounts[collection]
	ret = r.returned[collection]
	req = r.requested[collection]
	under = r.underfilled[collection]
	hist = r.latencyHist[collection]
	r.mu.RUnlock()

	if counts != nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock.
	if r.searchCounts[collection] != nil {
		return r.searchCounts[collection], r.returned[collection],
			r.requested[collection], r.underfilled[collection],
			r.latencyHist[collection]
	}

	counts = &[4]atomic.Uint64{}
	ret = &[4]atomic.Uint64{}
	req = &[4]atomic.Uint64{}
	under = &[4]atomic.Uint64{}
	hist = &[4]LatencyHistogram{}
	r.searchCounts[collection] = counts
	r.returned[collection] = ret
	r.requested[collection] = req
	r.underfilled[collection] = under
	r.latencyHist[collection] = hist
	return
}

// RecordSearch records a completed search. See addendum3 §1.1, §1.3, §1.4.
func (r *LiveRecorder) RecordSearch(collection string, mode Mode, returned, requested int, latency time.Duration) {
	ord := modeOrd(mode)
	counts, ret, req, under, hist := r.getOrCreate(collection)

	// §1.1: search count
	if r.cfg.SearchCount {
		counts[ord].Add(1)
	}

	// §1.3: hit rate
	if r.cfg.HitRate {
		ret[ord].Add(uint64(returned))
		req[ord].Add(uint64(requested))
		if returned < requested {
			under[ord].Add(1)
		}
	}

	// §1.4: latency buckets (off by default, M3)
	if r.cfg.LatencyEnabled {
		hist[ord].Record(latency)
	}
}

// RecordTerms records hashed query tokens. See addendum3 §1.2, M2.
func (r *LiveRecorder) RecordTerms(collection string, tokens []string) {
	if !r.cfg.KeywordsEnabled || len(tokens) == 0 {
		return
	}

	r.mu.RLock()
	tk := r.topK[collection]
	r.mu.RUnlock()

	if tk == nil {
		r.mu.Lock()
		if r.topK[collection] == nil {
			r.topK[collection] = NewTopKTerms(r.cfg.KeywordsTopK, r.cfg.KeywordsRaw)
		}
		tk = r.topK[collection]
		r.mu.Unlock()
	}

	tk.Record(tokens)
}

// IsEnabled returns true. See addendum3 M1.
func (r *LiveRecorder) IsEnabled() bool { return true }

// Snapshot returns a point-in-time snapshot. See addendum3 §3.
func (r *LiveRecorder) Snapshot(collection string) *Snapshot {
	snap := &Snapshot{
		SearchCounts: make(map[Mode]uint64),
		Returned:     make(map[Mode]uint64),
		Requested:    make(map[Mode]uint64),
		Underfilled:  make(map[Mode]uint64),
		Latency:      make(map[Mode]LatencySnapshot),
		TopTerms:     nil,
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	collections := []string{collection}
	if collection == "" {
		// Aggregate all collections.
		collections = make([]string, 0, len(r.searchCounts))
		for c := range r.searchCounts {
			collections = append(collections, c)
		}
	}

	for _, col := range collections {
		if counts, ok := r.searchCounts[col]; ok {
			for i, m := range AllModes {
				snap.SearchCounts[m] += counts[i].Load()
			}
		}
		if ret, ok := r.returned[col]; ok {
			for i, m := range AllModes {
				snap.Returned[m] += ret[i].Load()
			}
		}
		if req, ok := r.requested[col]; ok {
			for i, m := range AllModes {
				snap.Requested[m] += req[i].Load()
			}
		}
		if under, ok := r.underfilled[col]; ok {
			for i, m := range AllModes {
				snap.Underfilled[m] += under[i].Load()
			}
		}
		if hist, ok := r.latencyHist[col]; ok {
			for i, m := range AllModes {
				snap.Latency[m] = hist[i].Snapshot()
			}
		}
		if tk, ok := r.topK[col]; ok {
			snap.TopTerms = tk.Snapshot()
		}
	}

	// Compute total and fill rate.
	for _, m := range AllModes {
		snap.TotalSearches += snap.SearchCounts[m]
	}
	var totalRet, totalReq uint64
	for _, m := range AllModes {
		totalRet += snap.Returned[m]
		totalReq += snap.Requested[m]
	}
	if totalReq > 0 {
		snap.FillRate = float64(totalRet) / float64(totalReq)
	}

	return snap
}

// Reset clears all aggregates. See addendum3 §2.
func (r *LiveRecorder) Reset(collection string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if collection == "" {
		r.searchCounts = make(map[string]*[4]atomic.Uint64)
		r.returned = make(map[string]*[4]atomic.Uint64)
		r.requested = make(map[string]*[4]atomic.Uint64)
		r.underfilled = make(map[string]*[4]atomic.Uint64)
		r.latencyHist = make(map[string]*[4]LatencyHistogram)
		r.topK = make(map[string]*TopKTerms)
	} else {
		delete(r.searchCounts, collection)
		delete(r.returned, collection)
		delete(r.requested, collection)
		delete(r.underfilled, collection)
		delete(r.latencyHist, collection)
		delete(r.topK, collection)
	}
}

// NewRecorder creates the appropriate recorder based on config. See addendum3 M1.
func NewRecorder(cfg Config) Recorder {
	if !cfg.Enabled {
		return NopRecorder{}
	}
	return NewLiveRecorder(cfg)
}
