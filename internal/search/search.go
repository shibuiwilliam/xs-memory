// Package search implements the query planner, RRF fusion, and scoring.
// See design §8.
package search

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/xs-memory/xs-memory/internal/index/fts"
	"github.com/xs-memory/xs-memory/internal/index/vector"
	"github.com/xs-memory/xs-memory/internal/provider"
)

// Mode selects the search strategy. See design §8.1.
type Mode int

const (
	ModeFTS    Mode = iota // full-text search only
	ModeVector             // vector search only
	ModeHybrid             // FTS + vector with RRF fusion
)

// Query is a search request. See design §8.1.
type Query struct {
	Collection string
	Text       string
	Vector     []float32 // explicit vector (optional)
	Mode       Mode
	TopK       int
	RRFk       int // RRF k parameter, default 60
	Filter     Filter
	Scoring    ScoringOpts
}

// Filter specifies metadata filters.
type Filter struct {
	Type     string            // memory type filter
	Metadata map[string]string // key=value metadata filter
	After    time.Time         // only memories after this time
	Before   time.Time         // only memories before this time
}

// ScoringOpts controls agent memory scoring. See design §8.3.
type ScoringOpts struct {
	WRelevance  float64       // weight for relevance, default 0.6
	WRecency    float64       // weight for recency, default 0.25
	WImportance float64       // weight for importance, default 0.15
	HalfLife    time.Duration // recency half-life, default 72h
}

// DefaultScoringOpts returns the default scoring weights. See design §14.
func DefaultScoringOpts() ScoringOpts {
	return ScoringOpts{
		WRelevance:  0.6,
		WRecency:    0.25,
		WImportance: 0.15,
		HalfLife:    72 * time.Hour,
	}
}

// Result is a scored search result. See design App. A.
type Result struct {
	MemoryID  ulid.ULID
	ChunkText string
	Score     float64
	Subscores Subscores
}

// Subscores breaks down the final score. See design §8.3.
type Subscores struct {
	Relevance  float64
	Recency    float64
	Importance float64
	FTSRank    int
	VecRank    int
}

// Searcher orchestrates search across FTS and vector indexes.
type Searcher struct {
	ftsIndex *fts.Index
	vecIndex *vector.Index
	embedder provider.Embedder // may be nil
}

// NewSearcher creates a search orchestrator.
func NewSearcher(ftsIdx *fts.Index, vecIdx *vector.Index, embedder provider.Embedder) *Searcher {
	return &Searcher{
		ftsIndex: ftsIdx,
		vecIndex: vecIdx,
		embedder: embedder,
	}
}

// Search executes a search query. See design §8.
func (s *Searcher) Search(ctx context.Context, q Query) ([]Result, error) {
	if q.TopK <= 0 {
		q.TopK = 10
	}
	if q.RRFk <= 0 {
		q.RRFk = 60 // default per design §8.2
	}

	switch q.Mode {
	case ModeFTS:
		return s.searchFTS(q), nil
	case ModeVector:
		return s.searchVector(ctx, q)
	case ModeHybrid:
		return s.searchHybrid(ctx, q)
	default:
		return s.searchFTS(q), nil
	}
}

func (s *Searcher) searchFTS(q Query) []Result {
	if s.ftsIndex == nil {
		return nil
	}
	// Fetch more than TopK for scoring pipeline.
	ftsResults := s.ftsIndex.Search(q.Text, q.TopK*2, fts.DefaultBM25())
	results := make([]Result, len(ftsResults))
	for i, r := range ftsResults {
		results[i] = Result{
			MemoryID: r.DocID,
			Score:    r.Score,
			Subscores: Subscores{
				Relevance: r.Score,
				FTSRank:   i + 1,
			},
		}
	}
	if len(results) > q.TopK {
		results = results[:q.TopK]
	}
	return results
}

func (s *Searcher) searchVector(ctx context.Context, q Query) ([]Result, error) {
	if s.vecIndex == nil {
		return nil, nil
	}

	queryVec := q.Vector
	if queryVec == nil && s.embedder != nil && q.Text != "" {
		vecs, err := s.embedder.Embed(ctx, []string{q.Text})
		if err != nil {
			return nil, err
		}
		if len(vecs) > 0 {
			queryVec = vecs[0]
		}
	}
	if queryVec == nil {
		return nil, nil
	}

	vecResults := s.vecIndex.Search(queryVec, q.TopK*2)
	results := make([]Result, len(vecResults))
	for i, r := range vecResults {
		results[i] = Result{
			MemoryID: r.DocID,
			Score:    r.Score,
			Subscores: Subscores{
				Relevance: r.Score,
				VecRank:   i + 1,
			},
		}
	}
	if len(results) > q.TopK {
		results = results[:q.TopK]
	}
	return results, nil
}

// searchHybrid performs RRF fusion of FTS and vector results.
// See design §8.2: score(d) = Σ 1/(k + rank_i(d))
func (s *Searcher) searchHybrid(ctx context.Context, q Query) ([]Result, error) {
	ftsResults := s.searchFTS(Query{
		Text: q.Text,
		TopK: q.TopK * 2,
	})

	vecResults, err := s.searchVector(ctx, Query{
		Text:   q.Text,
		Vector: q.Vector,
		TopK:   q.TopK * 2,
	})
	if err != nil {
		// Degraded: fall back to FTS only.
		vecResults = nil
	}

	return rrfFuse(ftsResults, vecResults, q.RRFk, q.TopK), nil
}

// rrfFuse merges two ranked lists using Reciprocal Rank Fusion.
// See design §8.2: score(d) = Σ 1/(k + rank_i(d))
func rrfFuse(ftsResults, vecResults []Result, k, topK int) []Result {
	type fusedEntry struct {
		memoryID ulid.ULID
		score    float64
		ftsRank  int
		vecRank  int
	}

	fused := make(map[ulid.ULID]*fusedEntry)

	for i, r := range ftsResults {
		rank := i + 1
		e, ok := fused[r.MemoryID]
		if !ok {
			e = &fusedEntry{memoryID: r.MemoryID}
			fused[r.MemoryID] = e
		}
		e.score += 1.0 / float64(k+rank)
		e.ftsRank = rank
	}

	for i, r := range vecResults {
		rank := i + 1
		e, ok := fused[r.MemoryID]
		if !ok {
			e = &fusedEntry{memoryID: r.MemoryID}
			fused[r.MemoryID] = e
		}
		e.score += 1.0 / float64(k+rank)
		e.vecRank = rank
	}

	results := make([]Result, 0, len(fused))
	for _, e := range fused {
		results = append(results, Result{
			MemoryID: e.memoryID,
			Score:    e.score,
			Subscores: Subscores{
				Relevance: e.score,
				FTSRank:   e.ftsRank,
				VecRank:   e.vecRank,
			},
		})
	}

	// Stable sort by score descending, then by ID for determinism.
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		return results[i].MemoryID.String() < results[j].MemoryID.String()
	})

	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}
	return results
}

// RecencyScore computes exponential decay for recency scoring.
// See design §8.3: recency(now, accessed_at; half_life).
func RecencyScore(now, accessedAt time.Time, halfLife time.Duration) float64 {
	if halfLife <= 0 {
		return 1.0
	}
	elapsed := now.Sub(accessedAt).Seconds()
	if elapsed < 0 {
		elapsed = 0
	}
	lambda := math.Ln2 / halfLife.Seconds()
	return math.Exp(-lambda * elapsed)
}
