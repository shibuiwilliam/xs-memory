package xsmem_test

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"

	"github.com/xs-memory/xs-memory/xsmem"
)

func TestExportImportRoundtrip(t *testing.T) {
	dir := t.TempDir()
	origPath := filepath.Join(dir, "original.smem")

	s, err := xsmem.Open(origPath)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	ctx := context.Background()
	s.Remember(ctx, xsmem.RememberOpts{Content: "English memory"})
	s.Remember(ctx, xsmem.RememberOpts{Content: "日本語のメモリ"})

	// Export.
	var buf bytes.Buffer
	if err := s.Export(&buf); err != nil {
		t.Fatalf("Export: %v", err)
	}
	s.Close()

	// Import to new location.
	importPath := filepath.Join(dir, "imported.smem")
	if err := xsmem.Import(importPath, &buf); err != nil {
		t.Fatalf("Import: %v", err)
	}

	// Open imported store and verify.
	s2, err := xsmem.Open(importPath)
	if err != nil {
		t.Fatalf("Open imported: %v", err)
	}
	defer s2.Close()

	list, err := s2.List(ctx, "default")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("imported store has %d memories, want 2", len(list))
	}
}
