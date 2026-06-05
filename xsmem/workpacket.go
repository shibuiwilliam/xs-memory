package xsmem

import (
	"context"
	"fmt"

	"github.com/xs-memory/xs-memory/internal/storage"
)

// WorkPacket is the organization to-do list returned by SuggestOrganization.
// The host model walks this and calls back via write tools. See addendum §4.1.
type WorkPacket struct {
	DuplicateClusters []DuplicateCluster `json:"duplicate_clusters,omitempty"`
	Untagged          []Memory           `json:"untagged,omitempty"`
	Unlinked          []Memory           `json:"unlinked,omitempty"`
	EpisodicClusters  []EpisodicCluster  `json:"episodic_clusters,omitempty"`
}

// DuplicateCluster groups near-duplicate memories by vector similarity.
// See addendum §4.1: memory_find_duplicate_candidates.
type DuplicateCluster struct {
	Memories   []Memory `json:"memories"`
	Similarity float64  `json:"similarity"` // minimum pairwise similarity in cluster
}

// EpisodicCluster groups episodic memories that may be ready for consolidation.
type EpisodicCluster struct {
	Memories []Memory `json:"memories"`
	Topic    string   `json:"topic,omitempty"`
}

// FindDuplicateCandidates finds clusters of near-duplicate memories by vector similarity.
// Pure vector math, no LLM. See addendum §4.1.
func (s *Store) FindDuplicateCandidates(ctx context.Context, collection string, threshold float64) ([]DuplicateCluster, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.findDuplicateCandidatesLocked(collection, threshold)
}

// findDuplicateCandidatesLocked is the lock-free inner implementation.
func (s *Store) findDuplicateCandidatesLocked(collection string, threshold float64) ([]DuplicateCluster, error) {
	if collection == "" {
		collection = "default"
	}
	if threshold <= 0 {
		threshold = 0.85
	}

	mems, err := s.meta.ListMemories(collection)
	if err != nil {
		return nil, fmt.Errorf("xsmem: list for dup detection: %w", err)
	}
	if len(mems) < 2 {
		return nil, nil
	}

	vecIdx := s.vecIndexes[collection]
	if vecIdx == nil {
		return nil, nil // no vector index → can't detect duplicates
	}

	// O(n^2) pairwise comparison — sufficient for xs-memory scale.
	type memPair struct{ a, b int }
	var pairs []memPair

	for i := 0; i < len(mems); i++ {
		vec := s.embedForContent(mems[i].Content)
		if vec == nil {
			continue
		}
		results := vecIdx.Search(vec, len(mems))
		for _, r := range results {
			if r.DocID == mems[i].ID {
				continue
			}
			if r.Score >= threshold {
				for j := i + 1; j < len(mems); j++ {
					if mems[j].ID == r.DocID {
						pairs = append(pairs, memPair{i, j})
					}
				}
			}
		}
	}

	if len(pairs) == 0 {
		return nil, nil
	}

	// Union-find clustering.
	parent := make([]int, len(mems))
	for i := range parent {
		parent[i] = i
	}
	find := func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	for _, p := range pairs {
		ra, rb := find(p.a), find(p.b)
		if ra != rb {
			parent[ra] = rb
		}
	}

	groups := make(map[int][]int)
	for i := range mems {
		groups[find(i)] = append(groups[find(i)], i)
	}

	var clusters []DuplicateCluster
	for _, indices := range groups {
		if len(indices) < 2 {
			continue
		}
		cluster := DuplicateCluster{Similarity: threshold}
		for _, idx := range indices {
			cluster.Memories = append(cluster.Memories, toPublicMemory(mems[idx]))
		}
		clusters = append(clusters, cluster)
	}
	return clusters, nil
}

// embedForContent generates an embedding for content text, or nil if no embedder.
func (s *Store) embedForContent(content string) []float32 {
	if s.embedder == nil {
		return nil
	}
	vecs, err := s.embedder.Embed(context.Background(), []string{content})
	if err != nil || len(vecs) == 0 {
		return nil
	}
	return vecs[0]
}

// SuggestOrganization returns a work packet: the to-do list for host-delegated
// organization. Pure deterministic analysis, no LLM. See addendum §4.1.
func (s *Store) SuggestOrganization(ctx context.Context, collection string) (*WorkPacket, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if collection == "" {
		collection = "default"
	}

	mems, err := s.meta.ListMemories(collection)
	if err != nil {
		return nil, fmt.Errorf("xsmem: list for organization: %w", err)
	}

	wp := &WorkPacket{}

	for _, mem := range mems {
		pub := toPublicMemory(mem)

		// Untagged: no metadata tags.
		tags, _ := mem.Metadata["tags"]
		autoTags, _ := mem.Metadata["auto_tags"]
		if tags == nil && autoTags == nil {
			wp.Untagged = append(wp.Untagged, pub)
		}

		// Episodic memories as consolidation candidates.
		if mem.Type == Episodic {
			wp.EpisodicClusters = appendToEpisodicPool(wp.EpisodicClusters, pub)
		}
	}

	// Find duplicate candidates (reuses lock-free inner method).
	dupClusters, err := s.findDuplicateCandidatesLocked(collection, 0.85)
	if err == nil {
		wp.DuplicateClusters = dupClusters
	}

	return wp, nil
}

func appendToEpisodicPool(clusters []EpisodicCluster, mem Memory) []EpisodicCluster {
	if len(clusters) == 0 || len(clusters[len(clusters)-1].Memories) >= 10 {
		clusters = append(clusters, EpisodicCluster{Topic: "episodic group"})
	}
	clusters[len(clusters)-1].Memories = append(clusters[len(clusters)-1].Memories, mem)
	return clusters
}

// Merge combines N memories into one. The caller (host model) supplies the merged summary.
// Originals are tombstoned (soft delete per N7). Requires confirmed=true for safety (H5).
// See addendum §4.2.
func (s *Store) Merge(ctx context.Context, opts MergeOpts) (string, error) {
	if !opts.Confirmed {
		return "", fmt.Errorf("xsmem: merge requires confirmed=true (addendum H5)")
	}
	if len(opts.IDs) < 2 {
		return "", fmt.Errorf("xsmem: merge requires at least 2 memory IDs")
	}
	if opts.Summary == "" {
		return "", fmt.Errorf("xsmem: merge requires a summary")
	}

	// Store the merged summary as a new memory.
	newID, err := s.Remember(ctx, RememberOpts{
		Collection:  opts.Collection,
		Content:     opts.Summary,
		ContentType: "text/plain",
		Source:      "merge:" + opts.IDs[0],
		Type:        Semantic,
		Metadata: map[string]any{
			"merged_from": opts.IDs,
			"provenance":  "host-delegated-merge",
		},
		Importance: opts.Importance,
	})
	if err != nil {
		return "", fmt.Errorf("xsmem: store merged summary: %w", err)
	}

	// Tombstone originals (soft delete per N7).
	for _, id := range opts.IDs {
		if err := s.Forget(ctx, id, false); err != nil {
			s.logger.Warn("merge: failed to tombstone original", "id", id, "error", err)
		}
	}

	return newID, nil
}

// MergeOpts are the options for Merge. See addendum §4.2.
type MergeOpts struct {
	Collection string   // target collection
	IDs        []string // memory IDs to merge
	Summary    string   // host-model-written merged summary
	Confirmed  bool     // must be true — safety gate (H5)
	Importance float32
}

// Ensure storage import is used.
var _ = (*storage.Memory)(nil)
