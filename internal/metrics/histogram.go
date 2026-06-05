package metrics

import (
	"sync/atomic"
	"time"
)

// LatencyHistogram is a fixed-bucket histogram for search latency.
// Only bucket counts are stored — no timestamps, no per-call durations,
// no min/max. See addendum3 §1.4, M3.
//
// Buckets (ms): ≤1, ≤5, ≤10, ≤25, ≤50, ≤100, ≤250, ≤500, >500
type LatencyHistogram struct {
	buckets [9]atomic.Uint64
}

// latencyBoundaries are the upper bounds for each bucket in nanoseconds.
// The last bucket (>500ms) has no upper bound; anything beyond 500ms goes there.
var latencyBoundaries = [8]time.Duration{
	1 * time.Millisecond,
	5 * time.Millisecond,
	10 * time.Millisecond,
	25 * time.Millisecond,
	50 * time.Millisecond,
	100 * time.Millisecond,
	250 * time.Millisecond,
	500 * time.Millisecond,
}

// BucketLabels are human-readable labels for each bucket.
var BucketLabels = [9]string{
	"le_1ms", "le_5ms", "le_10ms", "le_25ms", "le_50ms",
	"le_100ms", "le_250ms", "le_500ms", "gt_500ms",
}

// Record places a latency observation into the correct bucket.
// Only the bucket index is incremented — no raw duration is stored. See addendum3 M3.
func (h *LatencyHistogram) Record(d time.Duration) {
	for i, bound := range latencyBoundaries {
		if d <= bound {
			h.buckets[i].Add(1)
			return
		}
	}
	h.buckets[8].Add(1) // >500ms overflow bucket
}

// LatencySnapshot is a read-only copy of histogram bucket counts.
// Contains only counts, never timestamps or sub-bucket durations. See addendum3 M3.
type LatencySnapshot struct {
	Buckets [9]uint64 `json:"buckets"` // indexed same as BucketLabels
	Total   uint64    `json:"total"`
}

// Snapshot returns a point-in-time copy. See addendum3 §3.
func (h *LatencyHistogram) Snapshot() LatencySnapshot {
	var s LatencySnapshot
	for i := range h.buckets {
		s.Buckets[i] = h.buckets[i].Load()
		s.Total += s.Buckets[i]
	}
	return s
}

// Reset zeroes all buckets.
func (h *LatencyHistogram) Reset() {
	for i := range h.buckets {
		h.buckets[i].Store(0)
	}
}
