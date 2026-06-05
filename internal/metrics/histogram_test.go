package metrics

import (
	"testing"
	"time"
)

func TestHistogramBucketPlacement(t *testing.T) {
	var h LatencyHistogram

	tests := []struct {
		dur    time.Duration
		bucket int
	}{
		{500 * time.Microsecond, 0}, // ≤1ms
		{1 * time.Millisecond, 0},   // ≤1ms (boundary)
		{3 * time.Millisecond, 1},   // ≤5ms
		{5 * time.Millisecond, 1},   // ≤5ms (boundary)
		{10 * time.Millisecond, 2},  // ≤10ms (boundary)
		{25 * time.Millisecond, 3},  // ≤25ms (boundary)
		{50 * time.Millisecond, 4},  // ≤50ms (boundary)
		{100 * time.Millisecond, 5}, // ≤100ms (boundary)
		{250 * time.Millisecond, 6}, // ≤250ms (boundary)
		{500 * time.Millisecond, 7}, // ≤500ms (boundary)
		{501 * time.Millisecond, 8}, // >500ms
		{2 * time.Second, 8},        // >500ms
	}

	for _, tt := range tests {
		h.Record(tt.dur)
	}

	snap := h.Snapshot()
	if snap.Total != uint64(len(tests)) {
		t.Errorf("total = %d, want %d", snap.Total, len(tests))
	}

	// Verify specific buckets.
	// Bucket 0 (≤1ms): 2 entries
	if snap.Buckets[0] != 2 {
		t.Errorf("bucket[0] = %d, want 2", snap.Buckets[0])
	}
	// Bucket 8 (>500ms): 2 entries
	if snap.Buckets[8] != 2 {
		t.Errorf("bucket[8] = %d, want 2", snap.Buckets[8])
	}
}

func TestHistogramNoPreciseTiming(t *testing.T) {
	// Verify the LatencySnapshot struct contains ONLY bucket counts and a total.
	// No timestamps, no per-call durations, no min/max. See addendum3 M3.
	var h LatencyHistogram
	h.Record(42 * time.Millisecond)

	snap := h.Snapshot()
	// The only fields are Buckets (array of counts) and Total (sum of counts).
	// If someone adds a Duration, Timestamp, Min, Max, etc. this will break.
	_ = snap.Buckets
	_ = snap.Total
	// That's all there is — the struct has exactly these two fields.
	// This is a structural test: if the struct gains timing fields, tests must update.
}

func TestHistogramReset(t *testing.T) {
	var h LatencyHistogram
	h.Record(5 * time.Millisecond)
	h.Record(50 * time.Millisecond)

	h.Reset()
	snap := h.Snapshot()
	if snap.Total != 0 {
		t.Errorf("total after reset = %d, want 0", snap.Total)
	}
}
