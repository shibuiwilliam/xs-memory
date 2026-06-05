package metrics

// Snapshot is a point-in-time view of all metrics. See addendum3 §3.
type Snapshot struct {
	// Search counts per mode. See addendum3 §1.1.
	TotalSearches uint64          `json:"total_searches"`
	SearchCounts  map[Mode]uint64 `json:"search_counts"`

	// Hit rate. See addendum3 §1.3.
	Returned    map[Mode]uint64 `json:"returned"`
	Requested   map[Mode]uint64 `json:"requested"`
	Underfilled map[Mode]uint64 `json:"underfilled"`
	FillRate    float64         `json:"fill_rate"` // Σ returned / Σ requested

	// Latency histograms (only populated when latency enabled). See addendum3 §1.4.
	Latency map[Mode]LatencySnapshot `json:"latency,omitempty"`

	// Top-K keyword terms. See addendum3 §1.2.
	TopTerms []TermEntry `json:"top_terms,omitempty"`

	// Mode distribution — derived from SearchCounts (M4). See addendum3 §1.5.
	ModeDistribution map[Mode]float64 `json:"mode_distribution,omitempty"`
}

// ComputeModeDistribution derives mode shares from SearchCounts. See addendum3 §1.5, M4.
func (s *Snapshot) ComputeModeDistribution() {
	if s.TotalSearches == 0 {
		s.ModeDistribution = nil
		return
	}
	s.ModeDistribution = make(map[Mode]float64, len(AllModes))
	for _, m := range AllModes {
		s.ModeDistribution[m] = float64(s.SearchCounts[m]) / float64(s.TotalSearches)
	}
}
