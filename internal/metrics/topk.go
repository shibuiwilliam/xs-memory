package metrics

import (
	"sort"
	"sync"
)

// TopKTerms tracks a bounded, decayed top-K term-frequency table of hashed
// analyzer tokens. See addendum3 §1.2, M2, M7.
type TopKTerms struct {
	mu       sync.Mutex
	maxK     int
	storeRaw bool // when true, tokens stored as-is; when false, they are already hashed
	counts   map[string]float64
}

// TermEntry is a single entry in the top-K snapshot. See addendum3 §1.2.
type TermEntry struct {
	Token string  `json:"token"` // hashed unless store_raw=true
	Count float64 `json:"count"`
}

// NewTopKTerms creates a bounded top-K tracker. See addendum3 §1.2, M7.
func NewTopKTerms(maxK int, storeRaw bool) *TopKTerms {
	if maxK <= 0 {
		maxK = 200
	}
	return &TopKTerms{
		maxK:     maxK,
		storeRaw: storeRaw,
		counts:   make(map[string]float64),
	}
}

// Record increments counts for the given tokens.
// Tokens are expected to already be hashed by the caller unless storeRaw is true.
// See addendum3 §1.2, M2.
func (t *TopKTerms) Record(tokens []string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, tok := range tokens {
		t.counts[tok]++
	}

	// Bound: evict smallest when exceeding maxK. See addendum3 M7.
	if len(t.counts) > t.maxK {
		t.evictSmallest()
	}
}

// Decay applies multiplicative decay to all entries. See addendum3 §1.2.
func (t *TopKTerms) Decay(factor float64) {
	t.mu.Lock()
	defer t.mu.Unlock()

	for k, v := range t.counts {
		t.counts[k] = v * factor
		if t.counts[k] < 0.01 {
			delete(t.counts, k)
		}
	}
}

// Snapshot returns a sorted copy of the top-K entries. See addendum3 §3.
func (t *TopKTerms) Snapshot() []TermEntry {
	t.mu.Lock()
	defer t.mu.Unlock()

	entries := make([]TermEntry, 0, len(t.counts))
	for tok, count := range t.counts {
		entries = append(entries, TermEntry{Token: tok, Count: count})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Count > entries[j].Count
	})
	return entries
}

// Len returns the number of tracked terms.
func (t *TopKTerms) Len() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.counts)
}

// Reset clears all entries.
func (t *TopKTerms) Reset() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.counts = make(map[string]float64)
}

func (t *TopKTerms) evictSmallest() {
	// Find the entry with the smallest count and remove it.
	var minKey string
	minVal := float64(1<<63 - 1)
	for k, v := range t.counts {
		if v < minVal {
			minVal = v
			minKey = k
		}
	}
	delete(t.counts, minKey)
}
