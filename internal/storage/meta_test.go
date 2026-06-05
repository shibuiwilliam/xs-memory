package storage

import (
	"path/filepath"
	"testing"
	"time"
)

func TestMetaStorePutGet(t *testing.T) {
	path := filepath.Join(t.TempDir(), "meta.db")
	m, err := OpenMeta(path)
	if err != nil {
		t.Fatalf("OpenMeta: %v", err)
	}
	defer m.Close()

	id := newTestULID(t)
	now := time.Now().Truncate(time.Second)
	mem := &Memory{
		ID:          id,
		Collection:  "default",
		Content:     "テスト内容です",
		ContentType: "text/plain",
		Type:        MemoryTypeSemantic,
		Importance:  0.8,
		CreatedAt:   now,
		UpdatedAt:   now,
		AccessedAt:  now,
	}

	if err := m.PutMemory(mem); err != nil {
		t.Fatalf("PutMemory: %v", err)
	}

	got, err := m.GetMemory(id)
	if err != nil {
		t.Fatalf("GetMemory: %v", err)
	}
	if got.Content != "テスト内容です" {
		t.Errorf("Content = %q, want テスト内容です", got.Content)
	}
	if got.Collection != "default" {
		t.Errorf("Collection = %q, want default", got.Collection)
	}
	if got.Type != MemoryTypeSemantic {
		t.Errorf("Type = %q, want semantic", got.Type)
	}
}

func TestMetaStoreSoftDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "meta.db")
	m, err := OpenMeta(path)
	if err != nil {
		t.Fatalf("OpenMeta: %v", err)
	}
	defer m.Close()

	id := newTestULID(t)
	mem := &Memory{ID: id, Collection: "test", Content: "to be deleted"}
	m.PutMemory(mem)

	// Soft delete (default per N7).
	if err := m.DeleteMemory(id, false); err != nil {
		t.Fatalf("DeleteMemory: %v", err)
	}

	// Should be tombstoned.
	if !m.IsTombstoned(id) {
		t.Error("expected tombstoned after soft delete")
	}

	// Get should still return the record (with Deleted flag).
	got, err := m.GetMemory(id)
	if err != nil {
		t.Fatalf("GetMemory after soft delete: %v", err)
	}
	if !got.Deleted {
		t.Error("expected Deleted=true after soft delete")
	}

	// ListMemories should exclude it.
	list, err := m.ListMemories("test")
	if err != nil {
		t.Fatalf("ListMemories: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("ListMemories returned %d, want 0 (tombstoned)", len(list))
	}
}

func TestMetaStoreHardDelete(t *testing.T) {
	path := filepath.Join(t.TempDir(), "meta.db")
	m, err := OpenMeta(path)
	if err != nil {
		t.Fatalf("OpenMeta: %v", err)
	}
	defer m.Close()

	id := newTestULID(t)
	mem := &Memory{ID: id, Collection: "test", Content: "permanently gone"}
	m.PutMemory(mem)

	if err := m.DeleteMemory(id, true); err != nil {
		t.Fatalf("DeleteMemory hard: %v", err)
	}

	// Should be gone entirely.
	_, err = m.GetMemory(id)
	if err == nil {
		t.Error("expected error after hard delete")
	}
}

func TestMetaStoreCollection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "meta.db")
	m, err := OpenMeta(path)
	if err != nil {
		t.Fatalf("OpenMeta: %v", err)
	}
	defer m.Close()

	cfg := CollectionConfig{
		Name:           "japanese",
		Analyzer:       "ja",
		EmbedderID:     "nomic-embed-text",
		EmbedDimension: 768,
	}
	if err := m.PutCollection(cfg); err != nil {
		t.Fatalf("PutCollection: %v", err)
	}

	got, err := m.GetCollection("japanese")
	if err != nil {
		t.Fatalf("GetCollection: %v", err)
	}
	if got.Analyzer != "ja" || got.EmbedDimension != 768 {
		t.Errorf("collection mismatch: %+v", got)
	}
}
