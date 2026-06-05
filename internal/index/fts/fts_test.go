package fts

import (
	"crypto/rand"
	"testing"

	"github.com/oklog/ulid/v2"

	"github.com/xs-memory/xs-memory/internal/analyzer"
)

func newID(t *testing.T) ulid.ULID {
	t.Helper()
	id, err := ulid.New(ulid.Now(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestFTSBasicEnglish(t *testing.T) {
	a := analyzer.NewEnAnalyzer()
	idx := NewIndex(a)

	doc1 := newID(t)
	doc2 := newID(t)
	doc3 := newID(t)

	idx.Add(doc1, "The quick brown fox jumps over the lazy dog")
	idx.Add(doc2, "A lazy cat sleeps all day long")
	idx.Add(doc3, "Quick foxes are clever animals")

	results := idx.Search("quick fox", 10, DefaultBM25())
	if len(results) == 0 {
		t.Fatal("expected results for 'quick fox'")
	}
	// doc1 and doc3 should rank highest (both have "quick" and/or "fox").
	if results[0].DocID != doc1 && results[0].DocID != doc3 {
		t.Errorf("top result should be doc1 or doc3, got %s", results[0].DocID)
	}
}

func TestFTSJapanese(t *testing.T) {
	a, err := analyzer.NewJaAnalyzer()
	if err != nil {
		t.Fatal(err)
	}
	idx := NewIndex(a)

	doc1 := newID(t)
	doc2 := newID(t)
	doc3 := newID(t)

	idx.Add(doc1, "東京タワーは東京都港区にある電波塔です")
	idx.Add(doc2, "京都の金閣寺は美しい寺院です")
	idx.Add(doc3, "東京スカイツリーは世界一高い電波塔です")

	results := idx.Search("東京 電波塔", 10, DefaultBM25())
	if len(results) == 0 {
		t.Fatal("expected results for '東京 電波塔'")
	}
	// doc1 and doc3 should rank above doc2 (both mention 東京 and 電波塔).
	topIDs := make(map[ulid.ULID]bool)
	for _, r := range results {
		topIDs[r.DocID] = true
	}
	if !topIDs[doc1] {
		t.Error("doc1 (東京タワー) should appear in results")
	}
	if !topIDs[doc3] {
		t.Error("doc3 (スカイツリー) should appear in results")
	}
}

func TestFTSBM25Ranking(t *testing.T) {
	// Golden test: document with more term matches should rank higher.
	a := analyzer.NewEnAnalyzer()
	idx := NewIndex(a)

	relevant := newID(t)
	lessRelevant := newID(t)

	idx.Add(relevant, "Go programming language Go compiler Go runtime")
	idx.Add(lessRelevant, "Python programming language interpreter")

	results := idx.Search("Go programming", 10, DefaultBM25())
	if len(results) < 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].DocID != relevant {
		t.Error("document with more Go mentions should rank first")
	}
	if results[0].Score <= results[1].Score {
		t.Error("first result should have higher score")
	}
}

func TestFTSRemove(t *testing.T) {
	a := analyzer.NewEnAnalyzer()
	idx := NewIndex(a)

	id := newID(t)
	idx.Add(id, "hello world")
	idx.Remove(id)

	results := idx.Search("hello", 10, DefaultBM25())
	if len(results) != 0 {
		t.Errorf("expected 0 results after remove, got %d", len(results))
	}
	if idx.DocCount() != 0 {
		t.Errorf("DocCount = %d, want 0", idx.DocCount())
	}
}

func TestFTSEmptyQuery(t *testing.T) {
	a := analyzer.NewEnAnalyzer()
	idx := NewIndex(a)
	idx.Add(newID(t), "hello world")

	results := idx.Search("", 10, DefaultBM25())
	if len(results) != 0 {
		t.Error("empty query should return no results")
	}
}

func TestFTSTopK(t *testing.T) {
	a := analyzer.NewEnAnalyzer()
	idx := NewIndex(a)

	for i := 0; i < 20; i++ {
		idx.Add(newID(t), "common term appears everywhere")
	}

	results := idx.Search("common term", 5, DefaultBM25())
	if len(results) != 5 {
		t.Errorf("topK=5 but got %d results", len(results))
	}
}

func TestFTSMixedJapaneseEnglish(t *testing.T) {
	a, err := analyzer.NewJaAnalyzer()
	if err != nil {
		t.Fatal(err)
	}
	idx := NewIndex(a)

	doc := newID(t)
	idx.Add(doc, "Go言語でプログラミングする方法")

	results := idx.Search("プログラミング", 10, DefaultBM25())
	if len(results) == 0 {
		t.Fatal("expected results for プログラミング")
	}
	if results[0].DocID != doc {
		t.Error("wrong document returned")
	}
}
