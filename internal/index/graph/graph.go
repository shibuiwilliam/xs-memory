// Package graph implements a small-scale knowledge graph with triple store.
// Uses bbolt with SPO/POS/OSP indexes for O(log n) traversal in any direction.
// See design §7.3.
package graph

import (
	"encoding/json"
	"fmt"
	"strings"

	bolt "go.etcd.io/bbolt"
)

// Triple is a knowledge graph edge (subject, predicate, object).
// See design §5.1.
type Triple struct {
	S      string  `json:"s"` // subject
	P      string  `json:"p"` // predicate
	O      string  `json:"o"` // object
	Weight float32 `json:"weight"`
	Source string  `json:"source"` // originating memory ID
}

// Bucket names for triple indexes. See design §7.3.
var (
	bucketSPO = []byte("triples_spo") // subject → predicate → object
	bucketPOS = []byte("triples_pos") // predicate → object → subject
	bucketOSP = []byte("triples_osp") // object → subject → predicate
)

// Graph is a triple store backed by bbolt. See design §7.3.
type Graph struct {
	db *bolt.DB
}

// Open opens or creates a graph store.
func Open(db *bolt.DB) (*Graph, error) {
	err := db.Update(func(tx *bolt.Tx) error {
		for _, b := range [][]byte{bucketSPO, bucketPOS, bucketOSP} {
			if _, err := tx.CreateBucketIfNotExists(b); err != nil {
				return fmt.Errorf("graph: create bucket %s: %w", b, err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &Graph{db: db}, nil
}

// tripleKey creates a composite key for indexed lookup.
func tripleKey(a, b, c string) []byte {
	return []byte(a + "\x00" + b + "\x00" + c)
}

// AddTriple stores a triple with all three indexes. See design §7.3.
func (g *Graph) AddTriple(t Triple) error {
	data, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("graph: marshal triple: %w", err)
	}

	return g.db.Update(func(tx *bolt.Tx) error {
		if err := tx.Bucket(bucketSPO).Put(tripleKey(t.S, t.P, t.O), data); err != nil {
			return err
		}
		if err := tx.Bucket(bucketPOS).Put(tripleKey(t.P, t.O, t.S), data); err != nil {
			return err
		}
		return tx.Bucket(bucketOSP).Put(tripleKey(t.O, t.S, t.P), data)
	})
}

// RemoveTriple removes a triple from all indexes.
func (g *Graph) RemoveTriple(s, p, o string) error {
	return g.db.Update(func(tx *bolt.Tx) error {
		tx.Bucket(bucketSPO).Delete(tripleKey(s, p, o))
		tx.Bucket(bucketPOS).Delete(tripleKey(p, o, s))
		tx.Bucket(bucketOSP).Delete(tripleKey(o, s, p))
		return nil
	})
}

// FindBySubject returns all triples where subject matches. See design §7.3.
func (g *Graph) FindBySubject(subject string) ([]Triple, error) {
	return g.scanPrefix(bucketSPO, subject+"\x00")
}

// FindByPredicate returns all triples where predicate matches.
func (g *Graph) FindByPredicate(predicate string) ([]Triple, error) {
	return g.scanPrefix(bucketPOS, predicate+"\x00")
}

// FindByObject returns all triples where object matches.
func (g *Graph) FindByObject(object string) ([]Triple, error) {
	return g.scanPrefix(bucketOSP, object+"\x00")
}

// FindBySubjectPredicate returns triples matching (subject, predicate, *).
func (g *Graph) FindBySubjectPredicate(subject, predicate string) ([]Triple, error) {
	return g.scanPrefix(bucketSPO, subject+"\x00"+predicate+"\x00")
}

// Expand performs N-hop graph expansion from a start entity.
// Returns all entities reachable within maxHops. See design §7.3.
func (g *Graph) Expand(startEntity string, maxHops int) ([]Triple, error) {
	visited := make(map[string]bool)
	var result []Triple
	frontier := []string{startEntity}

	for hop := 0; hop < maxHops && len(frontier) > 0; hop++ {
		var nextFrontier []string
		for _, entity := range frontier {
			if visited[entity] {
				continue
			}
			visited[entity] = true

			// Find triples where entity is subject.
			triples, err := g.FindBySubject(entity)
			if err != nil {
				return nil, err
			}
			for _, t := range triples {
				result = append(result, t)
				if !visited[t.O] {
					nextFrontier = append(nextFrontier, t.O)
				}
			}

			// Find triples where entity is object.
			triples, err = g.FindByObject(entity)
			if err != nil {
				return nil, err
			}
			for _, t := range triples {
				result = append(result, t)
				if !visited[t.S] {
					nextFrontier = append(nextFrontier, t.S)
				}
			}
		}
		frontier = nextFrontier
	}
	return result, nil
}

// Count returns the total number of triples.
func (g *Graph) Count() int {
	var count int
	g.db.View(func(tx *bolt.Tx) error {
		count = tx.Bucket(bucketSPO).Stats().KeyN
		return nil
	})
	return count
}

// scanPrefix scans a bucket for keys with the given prefix.
func (g *Graph) scanPrefix(bucket []byte, prefix string) ([]Triple, error) {
	var result []Triple
	err := g.db.View(func(tx *bolt.Tx) error {
		c := tx.Bucket(bucket).Cursor()
		pfx := []byte(prefix)
		for k, v := c.Seek(pfx); k != nil && strings.HasPrefix(string(k), prefix); k, v = c.Next() {
			var t Triple
			if err := json.Unmarshal(v, &t); err != nil {
				continue
			}
			result = append(result, t)
		}
		return nil
	})
	return result, err
}
