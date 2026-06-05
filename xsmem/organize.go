package xsmem

import (
	"context"
	"fmt"

	"github.com/xs-memory/xs-memory/internal/index/graph"
	"github.com/xs-memory/xs-memory/internal/organizer"
)

// Organize runs LLM organization jobs on memories. See design §10.
func (s *Store) Organize(ctx context.Context, collection string, jobs ...string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.organizer == nil {
		return fmt.Errorf("xsmem: no LLM configured for organize")
	}

	if collection == "" {
		collection = "default"
	}

	mems, err := s.meta.ListMemories(collection)
	if err != nil {
		return fmt.Errorf("xsmem: list for organize: %w", err)
	}

	jobSet := make(map[string]bool)
	for _, j := range jobs {
		jobSet[j] = true
	}
	if len(jobSet) == 0 {
		// Default: run all.
		jobSet["extract"] = true
		jobSet["autotag"] = true
		jobSet["importance"] = true
	}

	for _, mem := range mems {
		if jobSet["extract"] && s.graph != nil {
			result, err := s.organizer.ExtractEntities(ctx, mem.Content)
			if err != nil {
				s.logger.Warn("organize extract failed", "id", mem.ID, "error", err)
				continue
			}
			for _, rel := range result.Relations {
				s.graph.AddTriple(graph.Triple{
					S:      rel.Subject,
					P:      rel.Predicate,
					O:      rel.Object,
					Weight: rel.Weight,
					Source: mem.ID.String(),
				})
			}
		}

		if jobSet["autotag"] {
			result, err := s.organizer.AutoTag(ctx, mem.Content)
			if err != nil {
				s.logger.Warn("organize autotag failed", "id", mem.ID, "error", err)
				continue
			}
			if len(result.Tags) > 0 {
				if mem.Metadata == nil {
					mem.Metadata = make(map[string]any)
				}
				mem.Metadata["auto_tags"] = result.Tags
				if result.Type != "" {
					mem.Type = MemoryType(result.Type)
				}
				s.meta.PutMemory(mem)
			}
		}

		if jobSet["importance"] {
			result, err := s.organizer.ScoreImportance(ctx, mem.Content)
			if err != nil {
				s.logger.Warn("organize importance failed", "id", mem.ID, "error", err)
				continue
			}
			mem.Importance = result.Score
			s.meta.PutMemory(mem)
		}
	}

	return nil
}

// initOrganizer sets up the organizer if an LLM is configured.
func (s *Store) initOrganizer() {
	if s.cfg.LLM != nil {
		s.organizer = organizer.New(s.cfg.LLM, s.logger)
	}
}
