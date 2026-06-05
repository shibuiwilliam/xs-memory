package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestManifestReadWriteRoundtrip(t *testing.T) {
	dir := t.TempDir()
	storePath := filepath.Join(dir, "test.smem")
	os.MkdirAll(storePath, 0o755)

	// Reading non-existent manifest should return empty default.
	m, err := ReadManifest(storePath)
	if err != nil {
		t.Fatalf("ReadManifest (new): %v", err)
	}
	if m.Version != 1 || len(m.Collections) != 0 {
		t.Errorf("unexpected default manifest: %+v", m)
	}

	// Add a collection and write.
	m.Collections["test"] = CollectionConfig{
		Name:           "test",
		Analyzer:       "ja",
		EmbedderID:     "nomic-embed-text",
		EmbedDimension: 768,
	}
	if err := WriteManifest(storePath, m); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}

	// Read back.
	m2, err := ReadManifest(storePath)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if len(m2.Collections) != 1 {
		t.Fatalf("got %d collections, want 1", len(m2.Collections))
	}
	c := m2.Collections["test"]
	if c.Analyzer != "ja" || c.EmbedDimension != 768 {
		t.Errorf("collection mismatch: %+v", c)
	}
}
