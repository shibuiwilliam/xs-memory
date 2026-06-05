package graph

import (
	"path/filepath"
	"testing"

	bolt "go.etcd.io/bbolt"
)

func openTestGraph(t *testing.T) *Graph {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := bolt.Open(path, 0o644, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	g, err := Open(db)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestAddAndFind(t *testing.T) {
	g := openTestGraph(t)

	g.AddTriple(Triple{S: "Tokyo", P: "is_a", O: "City", Weight: 1.0, Source: "mem1"})
	g.AddTriple(Triple{S: "Tokyo", P: "located_in", O: "Japan", Weight: 1.0, Source: "mem1"})
	g.AddTriple(Triple{S: "Kyoto", P: "is_a", O: "City", Weight: 1.0, Source: "mem2"})

	// Find by subject.
	triples, err := g.FindBySubject("Tokyo")
	if err != nil {
		t.Fatal(err)
	}
	if len(triples) != 2 {
		t.Errorf("FindBySubject(Tokyo) = %d, want 2", len(triples))
	}

	// Find by object.
	triples, err = g.FindByObject("City")
	if err != nil {
		t.Fatal(err)
	}
	if len(triples) != 2 {
		t.Errorf("FindByObject(City) = %d, want 2", len(triples))
	}

	// Find by predicate.
	triples, err = g.FindByPredicate("is_a")
	if err != nil {
		t.Fatal(err)
	}
	if len(triples) != 2 {
		t.Errorf("FindByPredicate(is_a) = %d, want 2", len(triples))
	}

	// Find by subject+predicate.
	triples, err = g.FindBySubjectPredicate("Tokyo", "is_a")
	if err != nil {
		t.Fatal(err)
	}
	if len(triples) != 1 {
		t.Errorf("FindBySubjectPredicate(Tokyo, is_a) = %d, want 1", len(triples))
	}
}

func TestRemoveTriple(t *testing.T) {
	g := openTestGraph(t)

	g.AddTriple(Triple{S: "A", P: "knows", O: "B"})
	g.RemoveTriple("A", "knows", "B")

	triples, _ := g.FindBySubject("A")
	if len(triples) != 0 {
		t.Error("triple should be removed")
	}
}

func TestExpand(t *testing.T) {
	g := openTestGraph(t)

	// Build a small graph: A -> B -> C -> D
	g.AddTriple(Triple{S: "A", P: "knows", O: "B"})
	g.AddTriple(Triple{S: "B", P: "knows", O: "C"})
	g.AddTriple(Triple{S: "C", P: "knows", O: "D"})

	// 1-hop from A: should reach B.
	triples, err := g.Expand("A", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(triples) != 1 {
		t.Errorf("1-hop from A = %d triples, want 1", len(triples))
	}

	// 2-hop from A: should reach B and C.
	triples, err = g.Expand("A", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(triples) < 2 {
		t.Errorf("2-hop from A = %d triples, want >=2", len(triples))
	}
}

func TestGraphJapanese(t *testing.T) {
	g := openTestGraph(t)

	g.AddTriple(Triple{S: "東京タワー", P: "所在地", O: "東京都港区"})
	g.AddTriple(Triple{S: "東京タワー", P: "種類", O: "電波塔"})

	triples, err := g.FindBySubject("東京タワー")
	if err != nil {
		t.Fatal(err)
	}
	if len(triples) != 2 {
		t.Errorf("Japanese triples = %d, want 2", len(triples))
	}
}

func TestCount(t *testing.T) {
	g := openTestGraph(t)

	g.AddTriple(Triple{S: "A", P: "r", O: "B"})
	g.AddTriple(Triple{S: "C", P: "r", O: "D"})

	if g.Count() != 2 {
		t.Errorf("Count = %d, want 2", g.Count())
	}
}
