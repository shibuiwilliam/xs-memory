package vector

import (
	"crypto/rand"
	"math"
	"testing"

	"github.com/oklog/ulid/v2"
)

func BenchmarkFlatSearch(b *testing.B) {
	dim := 768
	idx := NewIndex(dim, Cosine, false)

	// Pre-populate with 10k vectors.
	for i := 0; i < 10000; i++ {
		id, _ := ulid.New(ulid.Now(), rand.Reader)
		vec := make([]float32, dim)
		for j := range vec {
			vec[j] = float32(math.Sin(float64(i*dim + j)))
		}
		idx.Add(id, vec)
	}

	query := make([]float32, dim)
	for j := range query {
		query[j] = float32(math.Cos(float64(j)))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx.Search(query, 10)
	}
}

func BenchmarkQuantizedSearch(b *testing.B) {
	dim := 768
	idx := NewIndex(dim, Cosine, true)

	for i := 0; i < 10000; i++ {
		id, _ := ulid.New(ulid.Now(), rand.Reader)
		vec := make([]float32, dim)
		for j := range vec {
			vec[j] = float32(math.Sin(float64(i*dim + j)))
		}
		idx.Add(id, vec)
	}

	query := make([]float32, dim)
	for j := range query {
		query[j] = float32(math.Cos(float64(j)))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx.Search(query, 10)
	}
}
