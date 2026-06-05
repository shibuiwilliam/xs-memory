package xsmem

import (
	"context"
	"fmt"

	"github.com/xs-memory/xs-memory/internal/index/graph"
)

// Triple is a public knowledge graph edge. See design §5.1.
type Triple struct {
	Subject   string  `json:"subject"`
	Predicate string  `json:"predicate"`
	Object    string  `json:"object"`
	Weight    float32 `json:"weight"`
	Source    string  `json:"source"`
}

// Link creates a graph edge. See design App. A.
func (s *Store) Link(ctx context.Context, t Triple) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.graph == nil {
		return fmt.Errorf("xsmem: graph not initialized")
	}

	return s.graph.AddTriple(graph.Triple{
		S:      t.Subject,
		P:      t.Predicate,
		O:      t.Object,
		Weight: t.Weight,
		Source: t.Source,
	})
}

// Unlink removes a graph edge.
func (s *Store) Unlink(ctx context.Context, subject, predicate, object string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.graph == nil {
		return fmt.Errorf("xsmem: graph not initialized")
	}

	return s.graph.RemoveTriple(subject, predicate, object)
}

// GraphExpand performs N-hop expansion from an entity. See design §7.3.
func (s *Store) GraphExpand(ctx context.Context, entity string, hops int) ([]Triple, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.graph == nil {
		return nil, nil
	}

	triples, err := s.graph.Expand(entity, hops)
	if err != nil {
		return nil, fmt.Errorf("xsmem: graph expand: %w", err)
	}

	result := make([]Triple, len(triples))
	for i, t := range triples {
		result[i] = Triple{
			Subject:   t.S,
			Predicate: t.P,
			Object:    t.O,
			Weight:    t.Weight,
			Source:    t.Source,
		}
	}
	return result, nil
}
