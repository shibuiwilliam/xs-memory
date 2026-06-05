package fts

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/oklog/ulid/v2"

	"github.com/xs-memory/xs-memory/internal/analyzer"
)

func BenchmarkFTSAdd(b *testing.B) {
	a := analyzer.NewEnAnalyzer()
	idx := NewIndex(a)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id, _ := ulid.New(ulid.Now(), rand.Reader)
		idx.Add(id, fmt.Sprintf("document number %d about search engines and information retrieval", i))
	}
}

func BenchmarkFTSSearch(b *testing.B) {
	a := analyzer.NewEnAnalyzer()
	idx := NewIndex(a)

	// Pre-populate.
	for i := 0; i < 10000; i++ {
		id, _ := ulid.New(ulid.Now(), rand.Reader)
		idx.Add(id, fmt.Sprintf("document %d about various topics including search and databases", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx.Search("search databases", 10, DefaultBM25())
	}
}

func BenchmarkFTSSearchJapanese(b *testing.B) {
	a, err := analyzer.NewJaAnalyzer()
	if err != nil {
		b.Fatal(err)
	}
	idx := NewIndex(a)

	// Pre-populate.
	texts := []string{
		"東京タワーは有名な観光地です",
		"京都の寺院は美しい建物です",
		"大阪の食べ物は美味しいです",
		"北海道の自然は壮大です",
	}
	for i := 0; i < 1000; i++ {
		id, _ := ulid.New(ulid.Now(), rand.Reader)
		idx.Add(id, texts[i%len(texts)])
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx.Search("東京 観光", 10, DefaultBM25())
	}
}
