// Package fts implements the inverted index with BM25 ranking.
// See design §7.1.
package fts

import (
	"math"
	"sort"
	"sync"

	"github.com/oklog/ulid/v2"

	"github.com/xs-memory/xs-memory/internal/analyzer"
)

// Index is an in-memory inverted index for full-text search.
// Each segment will have its own index; search merges across segments.
// See design §7.1.
type Index struct {
	mu       sync.RWMutex
	analyzer analyzer.Analyzer

	// postings maps term → set of (docID, term frequency).
	postings map[string][]Posting

	// docLengths maps docID → document length (in tokens).
	docLengths map[ulid.ULID]int

	// totalDocs is the number of documents indexed.
	totalDocs int

	// avgDocLen is the average document length.
	avgDocLen float64
}

// Posting is a single entry in a postings list.
type Posting struct {
	DocID ulid.ULID
	TF    int // term frequency in this document
}

// Result is a scored search result.
type Result struct {
	DocID ulid.ULID
	Score float64
}

// BM25Params holds BM25 tuning parameters. See design §7.1.
type BM25Params struct {
	K1 float64 // term saturation, default 1.2
	B  float64 // length normalization, default 0.75
}

// DefaultBM25 returns standard BM25 parameters.
func DefaultBM25() BM25Params {
	return BM25Params{K1: 1.2, B: 0.75}
}

// NewIndex creates a new FTS index with the given analyzer.
func NewIndex(a analyzer.Analyzer) *Index {
	return &Index{
		analyzer:   a,
		postings:   make(map[string][]Posting),
		docLengths: make(map[ulid.ULID]int),
	}
}

// Add indexes a document. See design §7.1.
func (idx *Index) Add(id ulid.ULID, text string) {
	tokens := idx.analyzer.Analyze(text)
	if len(tokens) == 0 {
		return
	}

	// Count term frequencies.
	tf := make(map[string]int)
	for _, t := range tokens {
		tf[t]++
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Remove old postings if re-indexing.
	if _, exists := idx.docLengths[id]; exists {
		idx.removeDocLocked(id)
	}

	idx.docLengths[id] = len(tokens)
	idx.totalDocs++
	idx.recalcAvgLocked()

	for term, count := range tf {
		idx.postings[term] = append(idx.postings[term], Posting{DocID: id, TF: count})
	}
}

// Remove removes a document from the index.
func (idx *Index) Remove(id ulid.ULID) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.removeDocLocked(id)
}

func (idx *Index) removeDocLocked(id ulid.ULID) {
	if _, ok := idx.docLengths[id]; !ok {
		return
	}
	delete(idx.docLengths, id)
	idx.totalDocs--
	idx.recalcAvgLocked()

	for term, posts := range idx.postings {
		filtered := posts[:0]
		for _, p := range posts {
			if p.DocID != id {
				filtered = append(filtered, p)
			}
		}
		if len(filtered) == 0 {
			delete(idx.postings, term)
		} else {
			idx.postings[term] = filtered
		}
	}
}

func (idx *Index) recalcAvgLocked() {
	if idx.totalDocs == 0 {
		idx.avgDocLen = 0
		return
	}
	total := 0
	for _, l := range idx.docLengths {
		total += l
	}
	idx.avgDocLen = float64(total) / float64(idx.totalDocs)
}

// Search performs BM25-ranked search. See design §7.1.
func (idx *Index) Search(query string, topK int, params BM25Params) []Result {
	queryTokens := idx.analyzer.Analyze(query)
	if len(queryTokens) == 0 {
		return nil
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if idx.totalDocs == 0 {
		return nil
	}

	// Score each document.
	scores := make(map[ulid.ULID]float64)
	for _, term := range queryTokens {
		posts, ok := idx.postings[term]
		if !ok {
			continue
		}
		// IDF = ln((N - df + 0.5) / (df + 0.5) + 1)
		df := float64(len(posts))
		n := float64(idx.totalDocs)
		idf := math.Log((n-df+0.5)/(df+0.5) + 1)

		for _, p := range posts {
			dl := float64(idx.docLengths[p.DocID])
			tf := float64(p.TF)
			// BM25 term score.
			num := tf * (params.K1 + 1)
			denom := tf + params.K1*(1-params.B+params.B*dl/idx.avgDocLen)
			scores[p.DocID] += idf * num / denom
		}
	}

	// Collect and sort by score descending.
	results := make([]Result, 0, len(scores))
	for id, score := range scores {
		results = append(results, Result{DocID: id, Score: score})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}
	return results
}

// DocCount returns the number of indexed documents.
func (idx *Index) DocCount() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.totalDocs
}
