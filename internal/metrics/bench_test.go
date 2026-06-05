package metrics

import (
	"testing"
	"time"
)

// BenchmarkNopRecorderSearch proves the disabled path incurs zero allocations
// and negligible overhead. See addendum3 M1.
func BenchmarkNopRecorderSearch(b *testing.B) {
	var r Recorder = NopRecorder{}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.RecordSearch("default", ModeHybrid, 10, 10, 5*time.Millisecond)
	}
}

// BenchmarkNopRecorderTerms proves term recording is zero-cost when disabled.
func BenchmarkNopRecorderTerms(b *testing.B) {
	var r Recorder = NopRecorder{}
	tokens := []string{"hash1", "hash2", "hash3"}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.RecordTerms("default", tokens)
	}
}

// BenchmarkLiveRecorderSearch measures overhead of the enabled path.
func BenchmarkLiveRecorderSearch(b *testing.B) {
	r := NewLiveRecorder(Config{
		Enabled:        true,
		SearchCount:    true,
		HitRate:        true,
		LatencyEnabled: true,
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.RecordSearch("default", ModeHybrid, 10, 10, 5*time.Millisecond)
	}
}
