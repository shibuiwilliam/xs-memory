package xsmem

import (
	"fmt"
	"os"
	"path/filepath"

	icache "github.com/xs-memory/xs-memory/internal/cache"
	"github.com/xs-memory/xs-memory/internal/tuning"
)

// StoreInfo contains full store and per-collection information.
type StoreInfo struct {
	Path            string             `json:"path"`
	AbsPath         string             `json:"abs_path"`
	ManifestVersion int                `json:"manifest_version"`
	Collections     []CollectionInfo   `json:"collections"`
	TotalMemories   int                `json:"total_memories"`
	BlockCache      CacheStatsInfo     `json:"block_cache"`
	ResultCache     icache.Stats       `json:"result_cache"`
	Tuning          tuning.TuningStats `json:"tuning"`
	DiskUsage       DiskUsageInfo      `json:"disk_usage"`
}

// CollectionInfo holds per-collection details.
type CollectionInfo struct {
	Name           string `json:"name"`
	Analyzer       string `json:"analyzer"`
	EmbedderID     string `json:"embedder_id"`
	EmbedDimension int    `json:"embed_dimension"`
	MemoryCount    int    `json:"memory_count"`
	FTSDocCount    int    `json:"fts_doc_count"`
	VecDocCount    int    `json:"vec_doc_count"`
}

// DiskUsageInfo reports on-disk size of the store.
type DiskUsageInfo struct {
	TotalBytes int64  `json:"total_bytes"`
	Total      string `json:"total"` // human-readable
	MetaDB     int64  `json:"meta_db_bytes"`
	WAL        int64  `json:"wal_bytes"`
	Segments   int64  `json:"segments_bytes"`
	Blobs      int64  `json:"blobs_bytes"`
}

// Info returns detailed store and per-collection information.
func (s *Store) Info() StoreInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	absPath, _ := filepath.Abs(s.path)

	cs := s.cache.Stats()
	allMems, _ := s.meta.ListMemories("")

	info := StoreInfo{
		Path:            s.path,
		AbsPath:         absPath,
		ManifestVersion: s.manifest.Version,
		TotalMemories:   len(allMems),
		BlockCache: CacheStatsInfo{
			CapacityMB: float64(cs.Capacity) / (1024 * 1024),
			UsedMB:     float64(cs.Used) / (1024 * 1024),
			Count:      cs.Count,
		},
		ResultCache: s.resultCache.Stats(),
		Tuning:      s.tuningStore.Stats(),
		DiskUsage:   s.computeDiskUsage(),
	}

	// Per-collection details.
	for _, col := range s.manifest.Collections {
		ci := CollectionInfo{
			Name:           col.Name,
			Analyzer:       col.Analyzer,
			EmbedderID:     col.EmbedderID,
			EmbedDimension: col.EmbedDimension,
		}

		// Count memories in this collection.
		mems, _ := s.meta.ListMemories(col.Name)
		ci.MemoryCount = len(mems)

		// Index stats.
		if ftsIdx, ok := s.ftsIndexes[col.Name]; ok {
			ci.FTSDocCount = ftsIdx.DocCount()
		}
		if vecIdx, ok := s.vecIndexes[col.Name]; ok {
			ci.VecDocCount = vecIdx.DocCount()
		}

		info.Collections = append(info.Collections, ci)
	}

	return info
}

func (s *Store) computeDiskUsage() DiskUsageInfo {
	du := DiskUsageInfo{}

	// meta.db
	if fi, err := os.Stat(filepath.Join(s.path, "meta.db")); err == nil {
		du.MetaDB = fi.Size()
	}

	// wal/
	du.WAL = dirSize(filepath.Join(s.path, "wal"))

	// segments/
	du.Segments = dirSize(filepath.Join(s.path, "segments"))

	// blobs/
	du.Blobs = dirSize(filepath.Join(s.path, "blobs"))

	du.TotalBytes = du.MetaDB + du.WAL + du.Segments + du.Blobs
	// Add manifest.json size.
	if fi, err := os.Stat(filepath.Join(s.path, "manifest.json")); err == nil {
		du.TotalBytes += fi.Size()
	}

	du.Total = humanBytes(du.TotalBytes)
	return du
}

func dirSize(path string) int64 {
	var total int64
	entries, err := os.ReadDir(path)
	if err != nil {
		return 0
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if fi, err := e.Info(); err == nil {
			total += fi.Size()
		}
	}
	return total
}

func humanBytes(b int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case b >= gb:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(gb))
	case b >= mb:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(mb))
	case b >= kb:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(kb))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
