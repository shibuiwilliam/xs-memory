package metrics

import (
	"testing"
	"time"
)

func TestNopRecorderInterface(t *testing.T) {
	var r Recorder = NopRecorder{}

	// All methods must be callable without panic.
	r.RecordSearch("test", ModeFTS, 5, 10, time.Millisecond)
	r.RecordTerms("test", []string{"foo", "bar"})
	r.Reset("")

	snap := r.Snapshot("")
	if snap == nil {
		t.Fatal("NopRecorder.Snapshot should return non-nil empty snapshot")
	}
	if snap.TotalSearches != 0 {
		t.Errorf("NopRecorder should have zero searches, got %d", snap.TotalSearches)
	}
	if r.IsEnabled() {
		t.Error("NopRecorder should report disabled")
	}
}

func TestNewRecorderDisabledReturnsNop(t *testing.T) {
	r := NewRecorder(Config{Enabled: false})
	if _, ok := r.(NopRecorder); !ok {
		t.Error("NewRecorder with Enabled=false should return NopRecorder")
	}
}

func TestNewRecorderEnabledReturnsLive(t *testing.T) {
	r := NewRecorder(Config{Enabled: true, SearchCount: true})
	if _, ok := r.(*LiveRecorder); !ok {
		t.Error("NewRecorder with Enabled=true should return *LiveRecorder")
	}
	if !r.IsEnabled() {
		t.Error("LiveRecorder should report enabled")
	}
}

func TestLiveRecorderSearchCounts(t *testing.T) {
	r := NewLiveRecorder(Config{
		Enabled:     true,
		SearchCount: true,
		HitRate:     true,
	})

	// Scripted sequence: 3 FTS, 2 hybrid, 1 vector on "default".
	for i := 0; i < 3; i++ {
		r.RecordSearch("default", ModeFTS, 5, 10, time.Millisecond)
	}
	for i := 0; i < 2; i++ {
		r.RecordSearch("default", ModeHybrid, 10, 10, time.Millisecond)
	}
	r.RecordSearch("default", ModeVector, 3, 10, time.Millisecond)

	snap := r.Snapshot("default")
	if snap.TotalSearches != 6 {
		t.Errorf("total = %d, want 6", snap.TotalSearches)
	}
	if snap.SearchCounts[ModeFTS] != 3 {
		t.Errorf("FTS = %d, want 3", snap.SearchCounts[ModeFTS])
	}
	if snap.SearchCounts[ModeHybrid] != 2 {
		t.Errorf("Hybrid = %d, want 2", snap.SearchCounts[ModeHybrid])
	}
	if snap.SearchCounts[ModeVector] != 1 {
		t.Errorf("Vector = %d, want 1", snap.SearchCounts[ModeVector])
	}
}

func TestHitRateAndUnderfilled(t *testing.T) {
	r := NewLiveRecorder(Config{
		Enabled:     true,
		SearchCount: true,
		HitRate:     true,
	})

	// Full fill.
	r.RecordSearch("c", ModeFTS, 10, 10, 0)
	// Underfilled.
	r.RecordSearch("c", ModeFTS, 3, 10, 0)
	r.RecordSearch("c", ModeHybrid, 0, 5, 0)

	snap := r.Snapshot("c")

	// FTS: returned 13, requested 20
	if snap.Returned[ModeFTS] != 13 {
		t.Errorf("FTS returned = %d, want 13", snap.Returned[ModeFTS])
	}
	if snap.Requested[ModeFTS] != 20 {
		t.Errorf("FTS requested = %d, want 20", snap.Requested[ModeFTS])
	}
	// FTS underfilled: 1 (only the second query)
	if snap.Underfilled[ModeFTS] != 1 {
		t.Errorf("FTS underfilled = %d, want 1", snap.Underfilled[ModeFTS])
	}
	// Hybrid underfilled: 1
	if snap.Underfilled[ModeHybrid] != 1 {
		t.Errorf("Hybrid underfilled = %d, want 1", snap.Underfilled[ModeHybrid])
	}
	// Overall fill rate: (13+0) / (20+5) = 13/25 = 0.52
	expectedFill := 13.0 / 25.0
	if snap.FillRate < expectedFill-0.01 || snap.FillRate > expectedFill+0.01 {
		t.Errorf("FillRate = %f, want ~%f", snap.FillRate, expectedFill)
	}
}

func TestModeDistribution(t *testing.T) {
	r := NewLiveRecorder(Config{Enabled: true, SearchCount: true})

	r.RecordSearch("c", ModeFTS, 5, 10, 0)
	r.RecordSearch("c", ModeFTS, 5, 10, 0)
	r.RecordSearch("c", ModeHybrid, 5, 10, 0)
	r.RecordSearch("c", ModeGrep, 5, 10, 0)

	snap := r.Snapshot("c")
	snap.ComputeModeDistribution()

	if snap.ModeDistribution == nil {
		t.Fatal("ModeDistribution should be computed")
	}

	// Total = 4: FTS=2 (50%), Hybrid=1 (25%), Grep=1 (25%), Vector=0 (0%)
	if snap.ModeDistribution[ModeFTS] != 0.5 {
		t.Errorf("FTS share = %f, want 0.5", snap.ModeDistribution[ModeFTS])
	}
	if snap.ModeDistribution[ModeHybrid] != 0.25 {
		t.Errorf("Hybrid share = %f, want 0.25", snap.ModeDistribution[ModeHybrid])
	}
	if snap.ModeDistribution[ModeGrep] != 0.25 {
		t.Errorf("Grep share = %f, want 0.25", snap.ModeDistribution[ModeGrep])
	}
	if snap.ModeDistribution[ModeVector] != 0.0 {
		t.Errorf("Vector share = %f, want 0.0", snap.ModeDistribution[ModeVector])
	}

	// Sum must equal 1.0.
	var sum float64
	for _, m := range AllModes {
		sum += snap.ModeDistribution[m]
	}
	if sum < 0.99 || sum > 1.01 {
		t.Errorf("sum of mode shares = %f, want 1.0", sum)
	}
}

func TestLatencyBucketsOffByDefault(t *testing.T) {
	r := NewLiveRecorder(Config{
		Enabled:        true,
		SearchCount:    true,
		LatencyEnabled: false, // off by default (M3)
	})

	r.RecordSearch("c", ModeFTS, 5, 10, 50*time.Millisecond)

	snap := r.Snapshot("c")
	ls := snap.Latency[ModeFTS]
	if ls.Total != 0 {
		t.Errorf("latency total = %d, want 0 when disabled", ls.Total)
	}
}

func TestLatencyBucketsWhenEnabled(t *testing.T) {
	r := NewLiveRecorder(Config{
		Enabled:        true,
		LatencyEnabled: true,
	})

	r.RecordSearch("c", ModeFTS, 5, 10, 500*time.Microsecond) // ≤1ms bucket
	r.RecordSearch("c", ModeFTS, 5, 10, 3*time.Millisecond)   // ≤5ms bucket
	r.RecordSearch("c", ModeFTS, 5, 10, 75*time.Millisecond)  // ≤100ms bucket
	r.RecordSearch("c", ModeFTS, 5, 10, 600*time.Millisecond) // >500ms bucket

	snap := r.Snapshot("c")
	ls := snap.Latency[ModeFTS]
	if ls.Total != 4 {
		t.Errorf("latency total = %d, want 4", ls.Total)
	}
	if ls.Buckets[0] != 1 { // ≤1ms
		t.Errorf("bucket[0] (≤1ms) = %d, want 1", ls.Buckets[0])
	}
	if ls.Buckets[1] != 1 { // ≤5ms
		t.Errorf("bucket[1] (≤5ms) = %d, want 1", ls.Buckets[1])
	}
	if ls.Buckets[5] != 1 { // ≤100ms
		t.Errorf("bucket[5] (≤100ms) = %d, want 1", ls.Buckets[5])
	}
	if ls.Buckets[8] != 1 { // >500ms
		t.Errorf("bucket[8] (>500ms) = %d, want 1", ls.Buckets[8])
	}
}

func TestReset(t *testing.T) {
	r := NewLiveRecorder(Config{Enabled: true, SearchCount: true, HitRate: true})

	r.RecordSearch("c1", ModeFTS, 5, 10, 0)
	r.RecordSearch("c2", ModeVector, 3, 10, 0)

	// Reset only c1.
	r.Reset("c1")
	snap1 := r.Snapshot("c1")
	if snap1.TotalSearches != 0 {
		t.Errorf("c1 after reset = %d, want 0", snap1.TotalSearches)
	}
	snap2 := r.Snapshot("c2")
	if snap2.TotalSearches != 1 {
		t.Errorf("c2 should be unaffected = %d, want 1", snap2.TotalSearches)
	}

	// Reset all.
	r.Reset("")
	snapAll := r.Snapshot("")
	if snapAll.TotalSearches != 0 {
		t.Errorf("after global reset = %d, want 0", snapAll.TotalSearches)
	}
}

func TestSnapshotAllCollections(t *testing.T) {
	r := NewLiveRecorder(Config{Enabled: true, SearchCount: true})

	r.RecordSearch("a", ModeFTS, 5, 10, 0)
	r.RecordSearch("b", ModeVector, 3, 10, 0)

	// Empty collection = aggregate all.
	snap := r.Snapshot("")
	if snap.TotalSearches != 2 {
		t.Errorf("total = %d, want 2", snap.TotalSearches)
	}
}
