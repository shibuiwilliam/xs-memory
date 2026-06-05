// Package vector implements flat vector search with int8 scalar quantization.
// MVP uses brute-force; HNSW deferred until benchmarks justify it (design §7.2, D4).
package vector

import (
	"math"
	"sort"
	"sync"

	"github.com/oklog/ulid/v2"
)

// Distance metric. See design §7.2.
type Distance int

const (
	Cosine Distance = iota
	Dot
)

// Index is a flat vector index. See design §7.2.
type Index struct {
	mu       sync.RWMutex
	dim      int
	distance Distance
	quantize bool // use int8 quantization

	// Stored vectors. For quantized mode, we store both for accuracy comparison.
	vectors map[ulid.ULID]vecEntry
}

type vecEntry struct {
	raw       []float32 // original (kept for re-ranking if needed)
	quantized []int8    // int8 quantized version
	scale     float32   // quantization scale factor
}

// Result is a scored vector search result.
type Result struct {
	DocID ulid.ULID
	Score float64 // similarity score (higher = more similar)
}

// NewIndex creates a flat vector index. See design §7.2.
func NewIndex(dim int, dist Distance, quantize bool) *Index {
	return &Index{
		dim:      dim,
		distance: dist,
		quantize: quantize,
		vectors:  make(map[ulid.ULID]vecEntry),
	}
}

// Add adds a vector for a document. See design §7.2.
func (idx *Index) Add(id ulid.ULID, vec []float32) {
	if len(vec) != idx.dim {
		return // dimension mismatch, skip silently
	}

	entry := vecEntry{raw: vec}
	if idx.quantize {
		entry.quantized, entry.scale = quantizeInt8(vec)
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.vectors[id] = entry
}

// Remove removes a vector.
func (idx *Index) Remove(id ulid.ULID) {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	delete(idx.vectors, id)
}

// Search performs brute-force top-k search. See design §7.2.
func (idx *Index) Search(query []float32, topK int) []Result {
	if len(query) != idx.dim {
		return nil
	}

	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var qQuantized []int8
	var qScale float32
	if idx.quantize {
		qQuantized, qScale = quantizeInt8(query)
	}

	results := make([]Result, 0, len(idx.vectors))
	for id, entry := range idx.vectors {
		var score float64
		if idx.quantize {
			score = idx.scoreFn(qQuantized, qScale, entry.quantized, entry.scale)
		} else {
			score = idx.scoreRaw(query, entry.raw)
		}
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

// scoreRaw computes similarity on float32 vectors.
func (idx *Index) scoreRaw(a, b []float32) float64 {
	switch idx.distance {
	case Cosine:
		return cosineSim(a, b)
	case Dot:
		return dotProduct(a, b)
	default:
		return cosineSim(a, b)
	}
}

// scoreFn computes approximate similarity on int8 quantized vectors.
func (idx *Index) scoreFn(qa []int8, sa float32, qb []int8, sb float32) float64 {
	switch idx.distance {
	case Cosine:
		return cosineSimInt8(qa, sa, qb, sb)
	case Dot:
		return dotProductInt8(qa, sa, qb, sb)
	default:
		return cosineSimInt8(qa, sa, qb, sb)
	}
}

// DocCount returns the number of indexed vectors.
func (idx *Index) DocCount() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.vectors)
}

// Dim returns the vector dimension.
func (idx *Index) Dim() int {
	return idx.dim
}

// Quantized returns whether int8 quantization is active.
func (idx *Index) Quantized() bool {
	return idx.quantize
}

// --- Quantization (int8 scalar quantization) ---
// See design §7.2: scalar quantization to int8.

// quantizeInt8 maps float32 values to int8 range [-127, 127].
// Returns quantized values and the scale factor for dequantization.
func quantizeInt8(v []float32) ([]int8, float32) {
	// Find max absolute value.
	maxAbs := float32(0)
	for _, val := range v {
		a := float32(math.Abs(float64(val)))
		if a > maxAbs {
			maxAbs = a
		}
	}

	if maxAbs == 0 {
		return make([]int8, len(v)), 0
	}

	scale := maxAbs / 127.0
	quantized := make([]int8, len(v))
	for i, val := range v {
		quantized[i] = int8(math.Round(float64(val / scale)))
	}
	return quantized, scale
}

// --- Distance functions ---

func dotProduct(a, b []float32) float64 {
	var sum float64
	for i := range a {
		sum += float64(a[i]) * float64(b[i])
	}
	return sum
}

func cosineSim(a, b []float32) float64 {
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	denom := math.Sqrt(normA) * math.Sqrt(normB)
	if denom == 0 {
		return 0
	}
	return dot / denom
}

func dotProductInt8(a []int8, sa float32, b []int8, sb float32) float64 {
	var sum int64
	for i := range a {
		sum += int64(a[i]) * int64(b[i])
	}
	return float64(sum) * float64(sa) * float64(sb)
}

func cosineSimInt8(a []int8, sa float32, b []int8, sb float32) float64 {
	var dot, normA, normB int64
	for i := range a {
		ai, bi := int64(a[i]), int64(b[i])
		dot += ai * bi
		normA += ai * ai
		normB += bi * bi
	}
	denom := math.Sqrt(float64(normA)) * math.Sqrt(float64(normB))
	if denom == 0 {
		return 0
	}
	return float64(dot) / denom
}
