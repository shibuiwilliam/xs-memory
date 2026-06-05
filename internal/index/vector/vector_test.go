package vector

import (
	"crypto/rand"
	"math"
	"testing"

	"github.com/oklog/ulid/v2"
)

func newID(t *testing.T) ulid.ULID {
	t.Helper()
	id, err := ulid.New(ulid.Now(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestFlatSearchExactTopK(t *testing.T) {
	idx := NewIndex(3, Cosine, false)

	// Three vectors: query is closest to v1.
	id1 := newID(t)
	id2 := newID(t)
	id3 := newID(t)

	idx.Add(id1, []float32{1, 0, 0})
	idx.Add(id2, []float32{0, 1, 0})
	idx.Add(id3, []float32{0, 0, 1})

	results := idx.Search([]float32{0.9, 0.1, 0}, 2)
	if len(results) != 2 {
		t.Fatalf("got %d results, want 2", len(results))
	}
	if results[0].DocID != id1 {
		t.Errorf("top result should be id1 (closest to query)")
	}
	// Verify cosine similarity is correct.
	// cos(query, id1) = 0.9/sqrt(0.82) ≈ 0.9938
	if results[0].Score < 0.99 {
		t.Errorf("expected high similarity, got %f", results[0].Score)
	}
}

func TestQuantizedSearchAccuracy(t *testing.T) {
	dim := 128
	idx := NewIndex(dim, Cosine, true) // quantized
	idxExact := NewIndex(dim, Cosine, false)

	// Generate some vectors.
	ids := make([]ulid.ULID, 100)
	for i := range ids {
		ids[i] = newID(t)
		vec := make([]float32, dim)
		for j := range vec {
			vec[j] = float32(i*dim+j) / float32(100*dim)
		}
		idx.Add(ids[i], vec)
		idxExact.Add(ids[i], vec)
	}

	query := make([]float32, dim)
	for j := range query {
		query[j] = float32(50*dim+j) / float32(100*dim)
	}

	qResults := idx.Search(query, 5)
	eResults := idxExact.Search(query, 5)

	if len(qResults) != 5 || len(eResults) != 5 {
		t.Fatalf("results: quantized=%d, exact=%d", len(qResults), len(eResults))
	}

	// The top result should match between quantized and exact.
	if qResults[0].DocID != eResults[0].DocID {
		t.Errorf("quantized top-1 (%s) != exact top-1 (%s)", qResults[0].DocID, eResults[0].DocID)
	}

	// Quantization error bound: score difference should be small.
	for i := 0; i < 5; i++ {
		diff := math.Abs(qResults[i].Score - eResults[i].Score)
		if diff > 0.05 {
			t.Errorf("result %d: score diff %.4f exceeds tolerance 0.05", i, diff)
		}
	}
}

func TestDotProduct(t *testing.T) {
	idx := NewIndex(3, Dot, false)

	id1 := newID(t)
	id2 := newID(t)
	idx.Add(id1, []float32{1, 2, 3})
	idx.Add(id2, []float32{0, 0, 1})

	results := idx.Search([]float32{1, 1, 1}, 2)
	if len(results) != 2 {
		t.Fatalf("got %d results", len(results))
	}
	// dot([1,2,3], [1,1,1]) = 6, dot([0,0,1], [1,1,1]) = 1
	if results[0].DocID != id1 {
		t.Error("id1 should rank first with dot product")
	}
	if math.Abs(results[0].Score-6.0) > 0.001 {
		t.Errorf("dot score = %f, want 6.0", results[0].Score)
	}
}

func TestQuantizeInt8(t *testing.T) {
	input := []float32{1.0, -0.5, 0.0, 0.25}
	q, scale := quantizeInt8(input)

	if len(q) != 4 {
		t.Fatalf("quantized len = %d", len(q))
	}
	// Max abs = 1.0, scale = 1.0/127
	if q[0] != 127 {
		t.Errorf("q[0] = %d, want 127", q[0])
	}
	if q[2] != 0 {
		t.Errorf("q[2] = %d, want 0", q[2])
	}
	_ = scale

	// Dequantize and check error.
	for i, val := range input {
		dequant := float64(q[i]) * float64(scale)
		if math.Abs(dequant-float64(val)) > float64(scale) {
			t.Errorf("dequant[%d] = %f, want ~%f", i, dequant, val)
		}
	}
}

func TestVectorRemove(t *testing.T) {
	idx := NewIndex(3, Cosine, false)
	id := newID(t)
	idx.Add(id, []float32{1, 0, 0})
	idx.Remove(id)

	results := idx.Search([]float32{1, 0, 0}, 10)
	if len(results) != 0 {
		t.Errorf("expected 0 results after remove, got %d", len(results))
	}
}

func TestDimensionMismatch(t *testing.T) {
	idx := NewIndex(3, Cosine, false)
	id := newID(t)
	// Wrong dimension should be silently ignored.
	idx.Add(id, []float32{1, 0})
	if idx.DocCount() != 0 {
		t.Error("wrong dimension vector should not be indexed")
	}

	// Wrong dimension query.
	idx.Add(id, []float32{1, 0, 0})
	results := idx.Search([]float32{1, 0}, 10)
	if len(results) != 0 {
		t.Error("wrong dimension query should return nil")
	}
}
